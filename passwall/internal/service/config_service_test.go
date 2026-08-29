package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"passwall/config"
	"passwall/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigServiceUsesFileDefaultsAndEnvironmentSecrets(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	service := NewConfigService(&fakeSystemConfigRepo{values: map[string]string{}})

	cfg, err := service.GetConfig()

	require.NoError(t, err)
	assert.Equal(t, 7, cfg.Concurrent)
	assert.True(t, cfg.Proxy.Enabled)
	assert.True(t, cfg.IPCheck.Enable)
	assert.True(t, cfg.DefaultSub.AutoUpdate)
	assert.Equal(t, "https://risk.example.test", cfg.IPCheck.IPInfo.Scamalytics.Host)
	assert.Equal(t, "audit-user", cfg.IPCheck.IPInfo.Scamalytics.User)
	assert.Equal(t, "audit-key", cfg.IPCheck.IPInfo.Scamalytics.APIKey)
}

func TestConfigServiceDatabaseOverridesFileButNotEnvironmentSecrets(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	service := NewConfigService(&fakeSystemConfigRepo{values: map[string]string{
		"concurrent": "19",
		"ip_check":   `{"enable":false,"ip_info":{"enable":false,"scamalytics":{"host":"db","user":"db","api_key":"db"}}}`,
	}})

	cfg, err := service.GetConfig()

	require.NoError(t, err)
	assert.Equal(t, 19, cfg.Concurrent)
	assert.False(t, cfg.IPCheck.Enable)
	assert.Equal(t, "https://risk.example.test", cfg.IPCheck.IPInfo.Scamalytics.Host)
	assert.Equal(t, "audit-user", cfg.IPCheck.IPInfo.Scamalytics.User)
	assert.Equal(t, "audit-key", cfg.IPCheck.IPInfo.Scamalytics.APIKey)
}

func TestConfigServiceReturnsRepositoryAndDecodeErrors(t *testing.T) {
	prepareConfigServiceTestEnv(t)

	_, err := NewConfigService(&fakeSystemConfigRepo{getAllErr: errors.New("read failed")}).GetConfig()
	require.ErrorContains(t, err, "read failed")

	_, err = NewConfigService(&fakeSystemConfigRepo{values: map[string]string{"concurrent": "{"}}).GetConfig()
	require.ErrorContains(t, err, "decode system config")
}

func TestConfigServiceUpdateDoesNotReloadAfterPersistenceFailure(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{}, setManyErr: errors.New("write failed")}
	scheduler := &fakeConfigScheduler{}
	statistics := &fakeConfigStatistics{}
	service := NewConfigService(repo)
	service.SetScheduler(scheduler)
	service.SetStatisticsService(statistics)

	err := service.UpdateConfig(map[string]interface{}{
		"concurrent": 11,
		"proxy":      map[string]interface{}{"enabled": true},
	})

	require.ErrorContains(t, err, "write failed")
	assert.False(t, scheduler.called)
	assert.Empty(t, statistics.restarts)
}

func TestConfigServiceUpdateRejectsWrongTypeBeforePersistence(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{"concurrent": "9"}}
	service := NewConfigService(repo)

	err := service.UpdateConfig(map[string]interface{}{"concurrent": "oops"})

	require.ErrorContains(t, err, `decode system config "concurrent"`)
	assert.Equal(t, "9", repo.values["concurrent"])
	cfg, err := service.GetConfig()
	require.NoError(t, err)
	assert.Equal(t, 9, cfg.Concurrent)
}

func TestConfigServiceUpdateRejectsInvalidCandidateBeforePersistence(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{"concurrent": "9"}}
	service := NewConfigService(repo)

	err := service.UpdateConfig(map[string]interface{}{"concurrent": 0})

	require.ErrorIs(t, err, ErrInvalidConfig)
	assert.Equal(t, "9", repo.values["concurrent"])
}

func TestConfigServiceUpdateReturnsSchedulerError(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{}}
	scheduler := &fakeConfigScheduler{err: errors.New("reload failed")}
	statistics := &fakeConfigStatistics{}
	service := NewConfigService(repo)
	service.SetScheduler(scheduler)
	service.SetStatisticsService(statistics)

	err := service.UpdateConfig(map[string]interface{}{"concurrent": 11})

	require.ErrorContains(t, err, "reload failed")
	assert.True(t, scheduler.called)
	assert.Equal(t, map[string]string{}, repo.values)
	assert.Empty(t, statistics.restarts)
}

func TestConfigServiceUpdateRollsBackDatabaseAndRuntime(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{"concurrent": "9"}}
	scheduler := &fakeConfigScheduler{}
	statistics := &fakeConfigStatistics{restartErrs: []error{errors.New("start failed"), nil}}
	service := NewConfigService(repo)
	service.SetScheduler(scheduler)
	service.SetStatisticsService(statistics)

	err := service.UpdateConfig(map[string]interface{}{"concurrent": 11})

	require.ErrorContains(t, err, "start failed")
	assert.Equal(t, "9", repo.values["concurrent"])
	require.Len(t, scheduler.configs, 2)
	assert.Equal(t, 11, scheduler.configs[0].Concurrent)
	assert.Equal(t, 9, scheduler.configs[1].Concurrent)
	require.Len(t, statistics.restarts, 2)
}

func TestConfigServiceUpdatePreservesRedactedSecrets(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{
		"proxy":     `{"enabled":true,"url":"http://proxy-secret"}`,
		"clash_api": `{"enable":true,"clients":[{"url":"ws://clash-secret","secret":"clash-secret"}]}`,
		"cron_jobs": `[{"name":"job","schedule":"0 0 0 1 1 *","webhook":[{"name":"hook","method":"POST","url":"https://hook-secret","header":"secret-header","body":"secret-body"}]}]`,
	}}
	service := NewConfigService(repo)

	err := service.UpdateConfig(map[string]interface{}{
		"proxy": map[string]interface{}{"enabled": false, "url": ""},
		"clash_api": map[string]interface{}{"enable": true, "clients": []map[string]interface{}{{
			"url": "", "secret": "", "url_configured": true, "secret_configured": true,
		}}},
		"cron_jobs": []map[string]interface{}{{
			"name": "job", "schedule": "0 0 0 1 1 *", "webhook": []map[string]interface{}{{
				"name": "hook", "method": "POST", "url": "", "header": "", "body": "",
			}},
		}},
	})

	require.NoError(t, err)
	assert.Contains(t, repo.values["proxy"], "http://proxy-secret")
	assert.Contains(t, repo.values["clash_api"], "ws://clash-secret")
	assert.Contains(t, repo.values["clash_api"], "clash-secret")
	assert.Contains(t, repo.values["cron_jobs"], "https://hook-secret")
	assert.Contains(t, repo.values["cron_jobs"], "secret-header")
	assert.Contains(t, repo.values["cron_jobs"], "secret-body")

	err = service.UpdateConfig(map[string]interface{}{
		"clash_api": map[string]interface{}{"enable": true, "clients": []map[string]interface{}{{
			"url": "ws://new-endpoint", "secret": "",
		}}},
		"cron_jobs": []map[string]interface{}{{
			"name": "job", "schedule": "0 0 0 1 1 *", "webhook": []map[string]interface{}{{
				"name": "hook", "method": "POST", "url": "https://new-endpoint", "header": "", "body": "",
			}},
		}},
	})
	require.NoError(t, err)
	assert.NotContains(t, repo.values["clash_api"], "clash-secret")
	assert.NotContains(t, repo.values["cron_jobs"], "secret-header")
	assert.NotContains(t, repo.values["cron_jobs"], "secret-body")
}

func TestConfigServiceUpdateDoesNotCarryOmittedSecretsToNewEndpoints(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{
		"clash_api": `{"enable":true,"clients":[{"url":"ws://old-endpoint","secret":"old-clash-secret"}]}`,
		"cron_jobs": `[{"name":"job","schedule":"0 0 0 1 1 *","webhook":[{"name":"hook","method":"POST","url":"https://old-endpoint","header":"old-header","body":"old-body"}]}]`,
	}}
	service := NewConfigService(repo)

	err := service.UpdateConfig(map[string]interface{}{
		"clash_api": map[string]interface{}{"enable": true, "clients": []map[string]interface{}{{
			"existing_index": 0,
			"url":            "ws://new-endpoint",
		}}},
		"cron_jobs": []map[string]interface{}{{
			"name": "job", "schedule": "0 0 0 1 1 *", "webhook": []map[string]interface{}{{
				"name": "hook", "method": "POST", "url": "https://new-endpoint",
			}},
		}},
	})

	require.NoError(t, err)
	var clashAPI config.ClashAPIConfig
	require.NoError(t, json.Unmarshal([]byte(repo.values["clash_api"]), &clashAPI))
	require.Len(t, clashAPI.Clients, 1)
	assert.Equal(t, "ws://new-endpoint", clashAPI.Clients[0].URL)
	assert.Empty(t, clashAPI.Clients[0].Secret)

	var cronJobs []config.CronJob
	require.NoError(t, json.Unmarshal([]byte(repo.values["cron_jobs"]), &cronJobs))
	require.Len(t, cronJobs, 1)
	require.Len(t, cronJobs[0].Webhook, 1)
	assert.Equal(t, "https://new-endpoint", cronJobs[0].Webhook[0].URL)
	assert.Empty(t, cronJobs[0].Webhook[0].Header)
	assert.Empty(t, cronJobs[0].Webhook[0].Body)
}

func TestConfigServiceUpdatePreservesClashClientByExistingIndex(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{
		"clash_api": `{"enable":true,"clients":[{"url":"ws://first","secret":"first-secret"},{"url":"ws://second","secret":"second-secret"}]}`,
	}}
	service := NewConfigService(repo)

	err := service.UpdateConfig(map[string]interface{}{
		"clash_api": map[string]interface{}{"enable": true, "clients": []map[string]interface{}{{
			"existing_index": 1,
		}}},
	})

	require.NoError(t, err)
	var clashAPI config.ClashAPIConfig
	require.NoError(t, json.Unmarshal([]byte(repo.values["clash_api"]), &clashAPI))
	require.Len(t, clashAPI.Clients, 1)
	assert.Equal(t, "ws://second", clashAPI.Clients[0].URL)
	assert.Equal(t, "second-secret", clashAPI.Clients[0].Secret)
}

func TestConfigServiceUpdatePreservesRenamedWebhookByExistingIndexes(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{
		"cron_jobs": `[{"name":"old-job","schedule":"0 0 0 1 1 *","webhook":[{"name":"old-hook","method":"POST","url":"https://hook-secret","header":"secret-header","body":"secret-body"}]}]`,
	}}
	service := NewConfigService(repo)

	err := service.UpdateConfig(map[string]interface{}{
		"cron_jobs": []map[string]interface{}{{
			"existing_index": 0,
			"name":           "new-job",
			"schedule":       "0 0 0 1 1 *",
			"webhook": []map[string]interface{}{{
				"existing_index": 0,
				"name":           "new-hook",
				"method":         "POST",
			}},
		}},
	})

	require.NoError(t, err)
	var cronJobs []config.CronJob
	require.NoError(t, json.Unmarshal([]byte(repo.values["cron_jobs"]), &cronJobs))
	require.Len(t, cronJobs, 1)
	require.Len(t, cronJobs[0].Webhook, 1)
	assert.Equal(t, "new-job", cronJobs[0].Name)
	assert.Equal(t, "new-hook", cronJobs[0].Webhook[0].Name)
	assert.Equal(t, "https://hook-secret", cronJobs[0].Webhook[0].URL)
	assert.Equal(t, "secret-header", cronJobs[0].Webhook[0].Header)
	assert.Equal(t, "secret-body", cronJobs[0].Webhook[0].Body)
}

func TestConfigServiceMigratesLegacyAutoBanUnitsOnce(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{
		"cron_jobs": `[{"name":"job","auto_ban":{"enable":true,"success_rate_threshold":0.5,"download_speed_threshold":100,"upload_speed_threshold":200}}]`,
	}}
	service := NewConfigService(repo)

	cfg, err := service.GetConfig()

	require.NoError(t, err)
	require.Len(t, cfg.CronJobs, 1)
	assert.Equal(t, float64(50), cfg.CronJobs[0].AutoBan.SuccessRateThreshold)
	assert.Equal(t, 100*1024, cfg.CronJobs[0].AutoBan.DownloadSpeedThreshold)
	assert.Equal(t, 200*1024, cfg.CronJobs[0].AutoBan.UploadSpeedThreshold)
	assert.Equal(t, 2, cfg.CronJobs[0].AutoBan.UnitVersion)

	cfg, err = service.GetConfig()
	require.NoError(t, err)
	assert.Equal(t, float64(50), cfg.CronJobs[0].AutoBan.SuccessRateThreshold)
	assert.Equal(t, 100*1024, cfg.CronJobs[0].AutoBan.DownloadSpeedThreshold)
}

func TestConfigServiceSerializesCompleteRuntimeUpdates(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{"concurrent": "9"}}
	scheduler := newBlockingConfigScheduler()
	service := NewConfigService(repo)
	service.SetScheduler(scheduler)

	firstDone := make(chan error, 1)
	go func() { firstDone <- service.UpdateConfig(map[string]interface{}{"concurrent": 11}) }()
	<-scheduler.started

	secondDone := make(chan error, 1)
	go func() { secondDone <- service.UpdateConfig(map[string]interface{}{"concurrent": 12}) }()
	assert.Equal(t, []int{11}, scheduler.concurrentValues())
	close(scheduler.release)
	require.NoError(t, <-firstDone)
	require.NoError(t, <-secondDone)

	assert.Equal(t, "12", repo.values["concurrent"])
	assert.Equal(t, []int{11, 12}, scheduler.concurrentValues())
}

func prepareConfigServiceTestEnv(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
concurrent: 7
database:
  driver: sqlite
  dsn: ":memory:"
proxy:
  enabled: true
  url: "http://127.0.0.1:7890"
ip_check:
  enable: true
  ip_info:
    enable: true
  app_unlock:
    enable: true
default_sub:
  auto_update: true
  interval: "0 0 4 * * *"
`), 0600))
	t.Setenv("CONFIG_PATH", path)
	t.Setenv("PASSWALL_TOKEN", "token")
	t.Setenv("SCAMALYTICS_HOST", "https://risk.example.test")
	t.Setenv("SCAMALYTICS_USER", "audit-user")
	t.Setenv("SCAMALYTICS_API_KEY", "audit-key")
}

type fakeSystemConfigRepo struct {
	repository.SystemConfigRepository
	values     map[string]string
	getAllErr  error
	setManyErr error
}

func (r *fakeSystemConfigRepo) GetAll() (map[string]string, error) {
	values := make(map[string]string, len(r.values))
	for key, value := range r.values {
		values[key] = value
	}
	return values, r.getAllErr
}

func (r *fakeSystemConfigRepo) SetMany(values map[string]string) error {
	if r.setManyErr != nil {
		return r.setManyErr
	}
	if r.values == nil {
		r.values = make(map[string]string)
	}
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *fakeSystemConfigRepo) RestoreAll(values map[string]string) error {
	r.values = make(map[string]string, len(values))
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

type fakeConfigScheduler struct {
	called      bool
	err         error
	validateErr error
	configs     []config.Config
}

func (s *fakeConfigScheduler) Validate(config.Config) error { return s.validateErr }

func (s *fakeConfigScheduler) Init(cfg config.Config) error {
	s.called = true
	s.configs = append(s.configs, cfg)
	return s.err
}

type fakeConfigStatistics struct {
	restarts    []config.ClashAPIConfig
	restartErrs []error
}

type blockingConfigScheduler struct {
	mu      sync.Mutex
	configs []config.Config
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingConfigScheduler() *blockingConfigScheduler {
	return &blockingConfigScheduler{started: make(chan struct{}), release: make(chan struct{})}
}

func (s *blockingConfigScheduler) Validate(config.Config) error { return nil }

func (s *blockingConfigScheduler) Init(cfg config.Config) error {
	s.mu.Lock()
	s.configs = append(s.configs, cfg)
	first := len(s.configs) == 1
	s.mu.Unlock()
	if first {
		s.once.Do(func() { close(s.started) })
		<-s.release
	}
	return nil
}

func (s *blockingConfigScheduler) concurrentValues() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := make([]int, len(s.configs))
	for i, cfg := range s.configs {
		values[i] = cfg.Concurrent
	}
	return values
}

func (s *fakeConfigStatistics) Start() error { return nil }

func (s *fakeConfigStatistics) Stop() error { return nil }

func (s *fakeConfigStatistics) Restart(cfg config.ClashAPIConfig) error {
	s.restarts = append(s.restarts, cfg)
	if len(s.restartErrs) > 0 {
		err := s.restartErrs[0]
		s.restartErrs = s.restartErrs[1:]
		return err
	}
	return nil
}
