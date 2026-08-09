package service

import (
	"encoding/json"
	"fmt"
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

	// 2. 加载数据库所有配置
	dbConfigs, err := s.repo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("get system configs from DB: %w", err)
	}

	// 3. 数据库只覆盖存在的动态配置，缺失项继续使用文件配置
	if err := applyDBConfig(baseConfig, dbConfigs); err != nil {
		return nil, err
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
	var fullConfig *config.Config
	var err error

	// 使用作用域缩小锁的范围
	err = func() error {
		s.mu.Lock()
		defer s.mu.Unlock()

		// 1. 先完成序列化，避免格式错误时产生部分写入
		serialized := make(map[string]string)
		for key, value := range updates {
			if allowedConfigKeys[key] {
				jsonBytes, err := json.Marshal(value)
				if err != nil {
					return fmt.Errorf("encode system config %q: %w", key, err)
				}
				serialized[key] = string(jsonBytes)
			}
		}
		if err := applyDBConfig(&config.Config{}, serialized); err != nil {
			return err
		}
		if err := s.repo.SetMany(serialized); err != nil {
			return fmt.Errorf("save system configs: %w", err)
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
		if err := s.scheduler.Init(*fullConfig); err != nil {
			return fmt.Errorf("reload scheduler: %w", err)
		}
	}

	// 4. 热重载流量统计服务
	if s.statService != nil {
		s.statService.Stop()
		if fullConfig.ClashAPI.Enable {
			go func() {
				if err := s.statService.Start(); err != nil {
					log.Errorln("Failed to start traffic statistics service: %v", err)
				}
			}()
		}
	}

	return nil
}
