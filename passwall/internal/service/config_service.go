package service

import (
	"encoding/json"
	"passwall/config"
	"passwall/internal/repository"
	"sync"

	"github.com/metacubex/mihomo/log"
)

// Scheduler 定义调度器接口，打破循环依赖
type Scheduler interface {
	Init(config config.Config) error
}

// StatisticsService 定义流量统计接口
type StatisticsService interface {
	Start() error
	Stop()
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

	// 2. 显式清空动态字段
	baseConfig.Concurrent = 0
	baseConfig.Proxy = config.Proxy{}
	baseConfig.IPCheck = config.IPCheckConfig{}
	baseConfig.ClashAPI = config.ClashAPIConfig{}
	baseConfig.CronJobs = nil
	baseConfig.DefaultSub = config.DefaultSubscriptionUpdateConfig{}

	// 3. 加载数据库所有配置
	dbConfigs, err := s.repo.GetAll()
	if err != nil {
		log.Errorln("Failed to get system configs from DB: %v", err)
		return baseConfig, nil
	}

	// 4. 合并配置
	if val, ok := dbConfigs["concurrent"]; ok {
		_ = json.Unmarshal([]byte(val), &baseConfig.Concurrent)
	}
	if val, ok := dbConfigs["proxy"]; ok {
		_ = json.Unmarshal([]byte(val), &baseConfig.Proxy)
	}
	if val, ok := dbConfigs["ip_check"]; ok {
		envScamalytics := baseConfig.IPCheck.IPInfo.Scamalytics
		if err := json.Unmarshal([]byte(val), &baseConfig.IPCheck); err == nil {
			baseConfig.IPCheck.IPInfo.Scamalytics = envScamalytics
		}
	}
	if val, ok := dbConfigs["clash_api"]; ok {
		_ = json.Unmarshal([]byte(val), &baseConfig.ClashAPI)
	}
	if val, ok := dbConfigs["cron_jobs"]; ok {
		_ = json.Unmarshal([]byte(val), &baseConfig.CronJobs)
	}
	if val, ok := dbConfigs["default_sub"]; ok {
		_ = json.Unmarshal([]byte(val), &baseConfig.DefaultSub)
	}

	return baseConfig, nil
}

func (s *configService) UpdateConfig(updates map[string]interface{}) error {
	var fullConfig *config.Config
	var err error

	// 使用作用域缩小锁的范围
	err = func() error {
		s.mu.Lock()
		defer s.mu.Unlock()

		// 1. 保存到数据库
		for key, value := range updates {
			if allowedConfigKeys[key] {
				jsonBytes, err := json.Marshal(value)
				if err != nil {
					continue
				}
				_ = s.repo.Set(key, string(jsonBytes))
			}
		}

		// 2. 获取更新后的完整配置
		fullConfig, err = s.getConfigInternal()
		return err
	}()

	if err != nil {
		return err
	}

	// 在锁之外执行热重载，避免长时间持有锁导致死锁或性能问题
	// 3. 热重载调度器
	if s.scheduler != nil {
		_ = s.scheduler.Init(*fullConfig)
	}

	// 4. 热重载流量统计服务
	if s.statService != nil {
		s.statService.Stop()
		if fullConfig.ClashAPI.Enable {
			// 这里已经是在 goroutine 之外了，但为了安全起见也可以继续使用 go
			go s.statService.Start()
		}
	}

	return nil
}
