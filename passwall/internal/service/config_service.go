package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"passwall/config"
	"passwall/internal/repository"
	"sync"
)

// Scheduler 定义调度器接口，打破循环依赖
type Scheduler interface {
	Validate(config config.Config) error
	Init(config config.Config) error
}

// StatisticsService 定义流量统计接口
type StatisticsService interface {
	Stop() error
	Restart(config.ClashAPIConfig) error
}

type ConfigService interface {
	GetConfig() (*config.Config, error)
	UpdateConfig(updates map[string]interface{}) error
	SetScheduler(scheduler Scheduler)
	SetStatisticsService(ss StatisticsService)
	GetClashClients() ([]config.ClashAPIClient, bool)
}

type configService struct {
	repo        repository.SystemConfigRepository
	scheduler   Scheduler
	statService StatisticsService
	mu          sync.RWMutex
	updateMu    sync.Mutex
}

func NewConfigService(repo repository.SystemConfigRepository) ConfigService {
	return &configService{
		repo: repo,
	}
}

func (s *configService) SetScheduler(scheduler Scheduler) {
	s.scheduler = scheduler
}

func (s *configService) SetStatisticsService(ss StatisticsService) {
	s.statService = ss
}

func (s *configService) GetClashClients() ([]config.ClashAPIClient, bool) {
	cfg, err := s.GetConfig()
	if err != nil {
		return nil, false
	}
	return cfg.ClashAPI.Clients, cfg.ClashAPI.Enable
}

// 允许的配置键列表
var allowedConfigKeys = map[string]bool{
	"concurrent":  true,
	"proxy":       true,
	"ip_check":    true,
	"clash_api":   true,
	"cron_jobs":   true,
	"default_sub": true,
}

var ErrInvalidConfig = errors.New("invalid configuration")

func (s *configService) GetConfig() (*config.Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getConfigInternal()
}

// getConfigInternal 内部获取配置方法，不加锁，供内部调用防止死锁
func (s *configService) getConfigInternal() (*config.Config, error) {
	// 1. 加载基础文件配置
	baseConfig, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}

	// 2. 加载数据库所有配置
	dbConfigs, err := s.repo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("get system configs from DB: %w", err)
	}

	// 3. 数据库只覆盖存在的动态配置，缺失项继续使用文件配置
	if err := applyDBConfig(baseConfig, dbConfigs); err != nil {
		return nil, err
	}
	if migrated, err := migrateLegacyAutoBanUnits(baseConfig.CronJobs); err != nil {
		return nil, err
	} else if _, persisted := dbConfigs["cron_jobs"]; migrated && persisted {
		encoded, err := json.Marshal(baseConfig.CronJobs)
		if err != nil {
			return nil, fmt.Errorf("encode migrated cron jobs: %w", err)
		}
		if err := s.repo.SetMany(map[string]string{"cron_jobs": string(encoded)}); err != nil {
			return nil, fmt.Errorf("save migrated cron jobs: %w", err)
		}
	}

	return baseConfig, nil
}

func applyDBConfig(baseConfig *config.Config, dbConfigs map[string]string) error {
	if err := unmarshalDBConfig(dbConfigs, "concurrent", &baseConfig.Concurrent); err != nil {
		return err
	}
	if err := unmarshalDBConfig(dbConfigs, "proxy", &baseConfig.Proxy); err != nil {
		return err
	}
	envScamalytics := baseConfig.IPCheck.IPInfo.Scamalytics
	if err := unmarshalDBConfig(dbConfigs, "ip_check", &baseConfig.IPCheck); err != nil {
		return err
	}
	baseConfig.IPCheck.IPInfo.Scamalytics = envScamalytics
	if err := unmarshalDBConfig(dbConfigs, "clash_api", &baseConfig.ClashAPI); err != nil {
		return err
	}
	if err := unmarshalDBConfig(dbConfigs, "cron_jobs", &baseConfig.CronJobs); err != nil {
		return err
	}
	if err := unmarshalDBConfig(dbConfigs, "default_sub", &baseConfig.DefaultSub); err != nil {
		return err
	}
	return nil
}

func unmarshalDBConfig(values map[string]string, key string, target interface{}) error {
	value, ok := values[key]
	if !ok {
		return nil
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return fmt.Errorf("decode system config %q: %w", key, err)
	}
	return nil
}

func (s *configService) UpdateConfig(updates map[string]interface{}) error {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()

	oldConfig, err := s.GetConfig()
	if err != nil {
		return err
	}
	oldValues, err := s.repo.GetAll()
	if err != nil {
		return fmt.Errorf("snapshot system configs: %w", err)
	}

	serializedUpdates := make(map[string]string)
	for key, value := range updates {
		if !allowedConfigKeys[key] {
			continue
		}
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("encode system config %q: %w", key, err)
		}
		serializedUpdates[key] = string(jsonBytes)
	}
	if len(serializedUpdates) == 0 {
		return nil
	}

	candidate := cloneConfig(*oldConfig)
	if _, updated := serializedUpdates["clash_api"]; updated {
		for i := range candidate.ClashAPI.Clients {
			candidate.ClashAPI.Clients[i].URL = ""
			candidate.ClashAPI.Clients[i].Secret = ""
		}
	}
	if _, updated := serializedUpdates["cron_jobs"]; updated {
		for i := range candidate.CronJobs {
			for j := range candidate.CronJobs[i].Webhook {
				candidate.CronJobs[i].Webhook[j].URL = ""
				candidate.CronJobs[i].Webhook[j].Header = ""
				candidate.CronJobs[i].Webhook[j].Body = ""
			}
		}
	}
	if err := applyDBConfig(&candidate, serializedUpdates); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	if _, err := migrateLegacyAutoBanUnits(candidate.CronJobs); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	if candidate.Concurrent <= 0 {
		return fmt.Errorf("%w: concurrent must be greater than zero", ErrInvalidConfig)
	}
	clientIndexes, hasClientIndexes := clashClientIndexes(serializedUpdates["clash_api"])
	jobIndexes, hasJobIndexes := cronJobIndexes(serializedUpdates["cron_jobs"])
	preserveConfigSecrets(&candidate, oldConfig, clientIndexes, hasClientIndexes, jobIndexes, hasJobIndexes)
	serialized, err := serializeUpdatedConfig(candidate, serializedUpdates)
	if err != nil {
		return err
	}
	if s.scheduler != nil {
		if err := s.scheduler.Validate(candidate); err != nil {
			return fmt.Errorf("%w: validate scheduler: %v", ErrInvalidConfig, err)
		}
	}

	s.mu.Lock()
	err = s.repo.SetMany(serialized)
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("save system configs: %w", err)
	}

	if s.scheduler != nil {
		if err := s.scheduler.Init(candidate); err != nil {
			return s.rollbackConfig(oldValues, oldConfig, fmt.Errorf("reload scheduler: %w", err), false)
		}
	}
	if s.statService != nil {
		if err := s.statService.Restart(candidate.ClashAPI); err != nil {
			return s.rollbackConfig(oldValues, oldConfig, fmt.Errorf("restart traffic statistics: %w", err), true)
		}
	}

	return nil
}

func cloneConfig(cfg config.Config) config.Config {
	cfg.ClashAPI.Clients = append([]config.ClashAPIClient(nil), cfg.ClashAPI.Clients...)
	cfg.CronJobs = append([]config.CronJob(nil), cfg.CronJobs...)
	for i := range cfg.CronJobs {
		cfg.CronJobs[i].Webhook = append([]config.WebhookConfig(nil), cfg.CronJobs[i].Webhook...)
	}
	return cfg
}

func (s *configService) rollbackConfig(oldValues map[string]string, oldConfig *config.Config, applyErr error, restoreRuntime bool) error {
	var rollbackErrs []error
	if restoreRuntime && s.scheduler != nil {
		if err := s.scheduler.Init(*oldConfig); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("restore scheduler: %w", err))
		}
	}
	if restoreRuntime && s.statService != nil {
		if err := s.statService.Restart(oldConfig.ClashAPI); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("restore traffic statistics: %w", err))
		}
	}
	s.mu.Lock()
	err := s.repo.RestoreAll(oldValues)
	s.mu.Unlock()
	if err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("restore system configs: %w", err))
	}
	if len(rollbackErrs) > 0 {
		return fmt.Errorf("config_apply_degraded: %w (rollback: %v)", applyErr, errors.Join(rollbackErrs...))
	}
	return applyErr
}

func preserveConfigSecrets(candidate *config.Config, old *config.Config, clientIndexes []int, hasClientIndexes bool, jobIndexes []cronJobIndex, hasJobIndexes bool) {
	if candidate.Proxy.URL == "" {
		candidate.Proxy.URL = old.Proxy.URL
	}
	for i := range candidate.ClashAPI.Clients {
		oldIndex := i
		if hasClientIndexes {
			if i >= len(clientIndexes) {
				continue
			}
			oldIndex = clientIndexes[i]
		}
		if oldIndex < 0 || oldIndex >= len(old.ClashAPI.Clients) {
			continue
		}
		client := &candidate.ClashAPI.Clients[i]
		oldClient := old.ClashAPI.Clients[oldIndex]
		if client.URL == "" {
			client.URL = oldClient.URL
		}
		if client.Secret == "" && client.URL == oldClient.URL {
			client.Secret = oldClient.Secret
		}
	}
	oldJobs := make(map[string]config.CronJob, len(old.CronJobs))
	for _, job := range old.CronJobs {
		oldJobs[job.Name] = job
	}
	for i := range candidate.CronJobs {
		oldJob, ok := oldJobs[candidate.CronJobs[i].Name]
		if hasJobIndexes {
			ok = i < len(jobIndexes) && jobIndexes[i].existingIndex >= 0 && jobIndexes[i].existingIndex < len(old.CronJobs)
			if ok {
				oldJob = old.CronJobs[jobIndexes[i].existingIndex]
			}
		}
		if !ok {
			continue
		}
		oldWebhooks := make(map[string]config.WebhookConfig, len(oldJob.Webhook))
		for _, webhook := range oldJob.Webhook {
			oldWebhooks[webhook.Name] = webhook
		}
		for j := range candidate.CronJobs[i].Webhook {
			webhook := &candidate.CronJobs[i].Webhook[j]
			oldWebhook, ok := oldWebhooks[webhook.Name]
			if hasJobIndexes && jobIndexes[i].hasWebhookIndexes {
				ok = j < len(jobIndexes[i].webhookIndexes) && jobIndexes[i].webhookIndexes[j] >= 0 && jobIndexes[i].webhookIndexes[j] < len(oldJob.Webhook)
				if ok {
					oldWebhook = oldJob.Webhook[jobIndexes[i].webhookIndexes[j]]
				}
			}
			if !ok {
				continue
			}
			if webhook.URL == "" {
				webhook.URL = oldWebhook.URL
			}
			if webhook.Header == "" && webhook.URL == oldWebhook.URL {
				webhook.Header = oldWebhook.Header
			}
			if webhook.Body == "" && webhook.URL == oldWebhook.URL {
				webhook.Body = oldWebhook.Body
			}
		}
	}
}

type cronJobIndex struct {
	existingIndex     int
	webhookIndexes    []int
	hasWebhookIndexes bool
}

func cronJobIndexes(serialized string) ([]cronJobIndex, bool) {
	if serialized == "" {
		return nil, false
	}
	var patch []struct {
		ExistingIndex *int `json:"existing_index"`
		Webhook       []struct {
			ExistingIndex *int `json:"existing_index"`
		} `json:"webhook"`
	}
	if err := json.Unmarshal([]byte(serialized), &patch); err != nil {
		return nil, false
	}
	indexes := make([]cronJobIndex, len(patch))
	hasIndexes := false
	for i, job := range patch {
		indexes[i].existingIndex = -1
		if job.ExistingIndex != nil {
			indexes[i].existingIndex = *job.ExistingIndex
			hasIndexes = true
		}
		indexes[i].webhookIndexes = make([]int, len(job.Webhook))
		for j, webhook := range job.Webhook {
			indexes[i].webhookIndexes[j] = -1
			if webhook.ExistingIndex != nil {
				indexes[i].webhookIndexes[j] = *webhook.ExistingIndex
				indexes[i].hasWebhookIndexes = true
			}
		}
	}
	return indexes, hasIndexes
}

func migrateLegacyAutoBanUnits(jobs []config.CronJob) (bool, error) {
	const bytesPerKilobyte = 1024
	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1
	migrated := false
	for i := range jobs {
		autoBan := &jobs[i].AutoBan
		if autoBan.UnitVersion >= config.AutoBanUnitVersion {
			continue
		}
		if autoBan.DownloadSpeedThreshold > maxInt/bytesPerKilobyte || autoBan.DownloadSpeedThreshold < minInt/bytesPerKilobyte ||
			autoBan.UploadSpeedThreshold > maxInt/bytesPerKilobyte || autoBan.UploadSpeedThreshold < minInt/bytesPerKilobyte {
			return false, fmt.Errorf("migrate cron job %q auto-ban units: speed threshold overflows int", jobs[i].Name)
		}
		autoBan.SuccessRateThreshold *= 100
		autoBan.DownloadSpeedThreshold *= bytesPerKilobyte
		autoBan.UploadSpeedThreshold *= bytesPerKilobyte
		autoBan.UnitVersion = config.AutoBanUnitVersion
		migrated = true
	}
	return migrated, nil
}

func clashClientIndexes(serialized string) ([]int, bool) {
	if serialized == "" {
		return nil, false
	}
	var patch struct {
		Clients []struct {
			ExistingIndex *int `json:"existing_index"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(serialized), &patch); err != nil {
		return nil, false
	}
	indexes := make([]int, len(patch.Clients))
	hasIndexes := false
	for i, client := range patch.Clients {
		indexes[i] = -1
		if client.ExistingIndex != nil {
			indexes[i] = *client.ExistingIndex
			hasIndexes = true
		}
	}
	return indexes, hasIndexes
}

func serializeUpdatedConfig(candidate config.Config, updated map[string]string) (map[string]string, error) {
	values := map[string]interface{}{
		"concurrent":  candidate.Concurrent,
		"proxy":       candidate.Proxy,
		"ip_check":    candidate.IPCheck,
		"clash_api":   candidate.ClashAPI,
		"cron_jobs":   candidate.CronJobs,
		"default_sub": candidate.DefaultSub,
	}
	ipCheck := candidate.IPCheck
	ipCheck.IPInfo.Scamalytics = config.Scamalytics{}
	values["ip_check"] = ipCheck

	result := make(map[string]string, len(updated))
	for key := range updated {
		encoded, err := json.Marshal(values[key])
		if err != nil {
			return nil, fmt.Errorf("encode system config %q: %w", key, err)
		}
		result[key] = string(encoded)
	}
	return result, nil
}
