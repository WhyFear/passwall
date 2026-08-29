package scheduler

import (
	"context"
	"errors"
	"fmt"
	"passwall/internal/model"
	"passwall/internal/service"
	"passwall/internal/service/proxy"
	"passwall/internal/service/task"
	"passwall/internal/util"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/metacubex/mihomo/log"
	"github.com/robfig/cron/v3"

	"passwall/config"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	cron            *cron.Cron
	reloadMutex     sync.Mutex
	jobMutex        sync.Mutex
	isRunning       bool
	stopContext     context.Context
	taskManager     task.TaskManager
	proxyTester     proxy.Tester
	subsManager     proxy.SubscriptionManager
	proxyService    proxy.ProxyService
	ipDetectService service.IPDetectorService
	jobIDs          map[string]cron.EntryID // 存储任务ID，用于更新
	jobExecutor     *cronJobExecutor

	configMutex   sync.RWMutex
	customConfigs map[uint]*model.SubscriptionConfig
	sysConfig     config.Config
}

type schedulerCandidate struct {
	cron          *cron.Cron
	jobIDs        map[string]cron.EntryID
	customConfigs map[uint]*model.SubscriptionConfig
}

const schedulerBatchSize = 500

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	return &Scheduler{
		cron:          newCron(),
		isRunning:     false,
		stopContext:   stopped,
		jobIDs:        make(map[string]cron.EntryID),
		customConfigs: make(map[uint]*model.SubscriptionConfig),
	}
}

func newCron() *cron.Cron {
	return cron.New(cron.WithSeconds(), cron.WithChain(
		cron.SkipIfStillRunning(cron.DefaultLogger),
		cron.Recover(cron.DefaultLogger),
	))
}

// SetServices 设置服务
func (s *Scheduler) SetServices(taskManager task.TaskManager,
	proxyTester proxy.Tester,
	subsManager proxy.SubscriptionManager,
	proxyService proxy.ProxyService,
	ipDetectService service.IPDetectorService,
) {
	s.taskManager = taskManager
	s.proxyTester = proxyTester
	s.subsManager = subsManager
	s.proxyService = proxyService
	s.ipDetectService = ipDetectService
	s.jobExecutor = newCronJobExecutor(taskManager, proxyTester, proxyService, ipDetectService)
}

// UpdateSubscriptionJob 更新订阅任务
func (s *Scheduler) UpdateSubscriptionJob(subID uint) error {
	s.reloadMutex.Lock()
	defer s.reloadMutex.Unlock()
	s.jobMutex.Lock()
	running := s.isRunning
	s.jobMutex.Unlock()
	if !running {
		return errors.New("scheduler is stopped")
	}

	subscription, err := s.subsManager.GetSubscriptionByID(subID)
	if err != nil {
		return err
	}

	s.configMutex.Lock()
	defer s.configMutex.Unlock()

	// 1. 获取最新配置
	subscriptionConfig, err := s.subsManager.GetSubscriptionConfig(subID)
	if err != nil {
		return err
	}
	if subscription == nil || subscription.Status == model.SubscriptionStatusDeleted {
		subscriptionConfig = nil
	}
	if subscriptionConfig != nil && subscriptionConfig.AutoUpdate && strings.TrimSpace(subscriptionConfig.UpdateInterval) == "" {
		return errors.New("subscription update interval is empty")
	}

	jobName := "sub_update_" + strconv.FormatUint(uint64(subID), 10)
	var newEntryID cron.EntryID
	if subscriptionConfig != nil && subscriptionConfig.AutoUpdate && subscriptionConfig.UpdateInterval != "" {
		newEntryID, err = addCustomSubJob(s.cron, s.subsManager, subID, subscriptionConfig, s.sysConfig.Proxy)
		if err != nil {
			return fmt.Errorf("add custom subscription job %s: %w", jobName, err)
		}
	}

	// 2. 更新内存映射
	if subscriptionConfig != nil {
		s.customConfigs[subID] = subscriptionConfig
	} else {
		delete(s.customConfigs, subID)
	}

	// 3. 处理 Cron 任务
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()

	// 先移除旧任务（如果存在）
	if entryID, exists := s.jobIDs[jobName]; exists {
		s.cron.Remove(entryID)
		delete(s.jobIDs, jobName)
		log.Infoln("Removed custom subscription update job %s", jobName)
	}

	// 如果有自定义配置且开启了自动更新，添加新任务
	if newEntryID != 0 {
		s.jobIDs[jobName] = newEntryID
		log.Infoln("Added custom subscription update job %s with schedule %s", jobName, subscriptionConfig.UpdateInterval)
	}
	return nil
}

// Validate 验证完整候选调度配置，不替换当前运行态。
func (s *Scheduler) Validate(sysConfig config.Config) error {
	s.reloadMutex.Lock()
	defer s.reloadMutex.Unlock()
	_, err := s.buildCandidate(sysConfig)
	return err
}

// Init 启动调度器
func (s *Scheduler) Init(sysConfig config.Config) error {
	s.reloadMutex.Lock()
	defer s.reloadMutex.Unlock()

	candidate, err := s.buildCandidate(sysConfig)
	if err != nil {
		return err
	}

	s.jobMutex.Lock()
	oldCron := s.cron
	wasRunning := s.isRunning
	s.isRunning = false
	s.jobMutex.Unlock()
	if wasRunning {
		<-oldCron.Stop().Done()
	}

	s.jobMutex.Lock()
	s.configMutex.Lock()
	s.cron = candidate.cron
	s.jobIDs = candidate.jobIDs
	s.customConfigs = candidate.customConfigs
	s.sysConfig = sysConfig
	s.configMutex.Unlock()
	s.cron.Start()
	s.isRunning = true
	s.stopContext = nil
	s.jobMutex.Unlock()
	return nil
}

func (s *Scheduler) buildCandidate(sysConfig config.Config) (*schedulerCandidate, error) {
	if s.jobExecutor == nil || s.subsManager == nil {
		return nil, errors.New("scheduler services are not set")
	}

	candidate := &schedulerCandidate{
		cron:          newCron(),
		jobIDs:        make(map[string]cron.EntryID),
		customConfigs: make(map[uint]*model.SubscriptionConfig),
	}
	jobNames := make(map[string]struct{}, len(sysConfig.CronJobs))

	// 添加任务
	for _, job := range sysConfig.CronJobs {
		if strings.TrimSpace(job.Name) == "" || job.Name == "default_sub_update" || strings.HasPrefix(job.Name, "sub_update_") {
			return nil, fmt.Errorf("invalid or reserved cron job name %q", job.Name)
		}
		if _, exists := jobNames[job.Name]; exists {
			return nil, fmt.Errorf("duplicate cron job name %q", job.Name)
		}
		jobNames[job.Name] = struct{}{}

		// 创建任务闭包
		jobConfig := job // 创建副本避免闭包问题
		entryID, err := candidate.cron.AddFunc(jobConfig.Schedule, func() {
			s.jobExecutor.Execute(jobConfig)
		})

		if err != nil {
			return nil, fmt.Errorf("add cron job %q: %w", job.Name, err)
		}

		// 存储任务ID
		candidate.jobIDs[job.Name] = entryID
	}

	// 处理订阅更新任务
	// 1. 获取所有订阅自定义配置
	customConfigs, err := s.subsManager.GetAllSubscriptionConfigs()
	if err != nil {
		return nil, fmt.Errorf("get subscription configs: %w", err)
	}

	for _, cfg := range customConfigs {
		// 验证该配置对应的订阅是否未被删除
		sub, err := s.subsManager.GetSubscriptionByID(cfg.SubscriptionID)
		if errors.Is(err, proxy.ErrSubscriptionNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get subscription %d: %w", cfg.SubscriptionID, err)
		}
		if sub == nil || sub.Status == model.SubscriptionStatusDeleted {
			continue
		}
		if _, exists := candidate.customConfigs[cfg.SubscriptionID]; exists {
			return nil, fmt.Errorf("duplicate subscription config %d", cfg.SubscriptionID)
		}
		candidate.customConfigs[cfg.SubscriptionID] = cfg
	}

	// 2. 注册有个性化配置的任务
	// 注意：这里我们只处理 Init 时的状态。UpdateSubscriptionJob 会处理运行时的变化。
	// 为了复用代码，UpdateSubscriptionJob 需要能够创建任务。
	// 但 Init 这里有 sysConfig 上下文。

	for subID, subCfg := range candidate.customConfigs {
		if subCfg.AutoUpdate && strings.TrimSpace(subCfg.UpdateInterval) == "" {
			return nil, fmt.Errorf("subscription %d update interval is empty", subID)
		}
		if subCfg.AutoUpdate {
			entryID, err := addCustomSubJob(candidate.cron, s.subsManager, subID, subCfg, sysConfig.Proxy)
			if err != nil {
				return nil, fmt.Errorf("add custom subscription job %d: %w", subID, err)
			}
			candidate.jobIDs["sub_update_"+strconv.FormatUint(uint64(subID), 10)] = entryID
		}
	}

	// 3. 处理默认订阅更新任务（针对没有自定义配置的订阅）
	if sysConfig.DefaultSub.AutoUpdate && strings.TrimSpace(sysConfig.DefaultSub.Interval) == "" {
		return nil, errors.New("default subscription update interval is empty")
	}
	if sysConfig.DefaultSub.AutoUpdate {
		entryID, err := candidate.cron.AddFunc(sysConfig.DefaultSub.Interval, func() {
			ctx := context.Background()
			log.Infoln("Executing default subscription update job (filtered)")

			// 构造下载选项
			var opts *util.DownloadOptions
			if sysConfig.DefaultSub.UseProxy && sysConfig.Proxy.Enabled && sysConfig.Proxy.URL != "" {
				opts = &util.DownloadOptions{
					ProxyURL: sysConfig.Proxy.URL,
				}
			}

			for afterID := uint(0); ; {
				subscriptions, err := s.subsManager.GetSubscriptionsAfterID(afterID, schedulerBatchSize)
				if err != nil {
					log.Errorln("Failed to get subscriptions for default update: %v", err)
					return
				}
				for _, sub := range subscriptions {
					s.configMutex.RLock()
					_, hasCustom := s.customConfigs[sub.ID]
					s.configMutex.RUnlock()
					if !hasCustom {
						if err := s.subsManager.RefreshSubscriptionAsync(ctx, sub.ID, opts); err != nil {
							log.Errorln("Default subscription update failed for sub %d: %v", sub.ID, err)
						}
					}
				}
				if len(subscriptions) < schedulerBatchSize {
					return
				}
				afterID = subscriptions[len(subscriptions)-1].ID
			}
		})

		if err != nil {
			return nil, fmt.Errorf("add default subscription update job: %w", err)
		}
		candidate.jobIDs["default_sub_update"] = entryID
	}
	return candidate, nil
}

func addCustomSubJob(target *cron.Cron, subsManager proxy.SubscriptionManager, subID uint, subCfg *model.SubscriptionConfig, proxyConfig config.Proxy) (cron.EntryID, error) {
	// 使用闭包捕获
	return target.AddFunc(subCfg.UpdateInterval, func() {
		ctx := context.Background()
		log.Infoln("Executing custom subscription update job for sub %d", subID)

		var opts *util.DownloadOptions
		if subCfg.UseProxy && proxyConfig.Enabled && proxyConfig.URL != "" {
			opts = &util.DownloadOptions{
				ProxyURL: proxyConfig.URL,
			}
		}

		if err := subsManager.RefreshSubscriptionAsync(ctx, subID, opts); err != nil {
			log.Errorln("Custom subscription update failed for sub %d: %v", subID, err)
		}
	})
}

// BeginStop 停止接受新的 cron 任务并返回正在执行任务的完成 context。
func (s *Scheduler) BeginStop() context.Context {
	s.reloadMutex.Lock()
	defer s.reloadMutex.Unlock()

	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()
	if !s.isRunning {
		return s.stopContext
	}
	s.isRunning = false
	s.stopContext = s.cron.Stop()
	return s.stopContext
}

// Stop 停止调度器并等待正在执行的任务完成。
func (s *Scheduler) Stop() {
	<-s.BeginStop().Done()
	log.Infoln("Scheduler stopped")
}

// GetStatus 获取调度器状态
func (s *Scheduler) GetStatus() map[string]interface{} {
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()

	status := make(map[string]interface{})
	status["is_running"] = s.isRunning

	// 获取所有任务的状态
	jobs := make(map[string]interface{})
	for name, id := range s.jobIDs {
		entry := s.cron.Entry(id)
		jobStatus := make(map[string]interface{})
		jobStatus["next_run"] = entry.Next.Format(time.RFC3339)
		jobStatus["prev_run"] = entry.Prev.Format(time.RFC3339)
		jobs[name] = jobStatus
	}

	status["jobs"] = jobs
	return status
}
