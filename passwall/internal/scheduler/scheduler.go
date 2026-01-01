package scheduler

import (
	"context"
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
	jobMutex        sync.Mutex
	isRunning       bool
	taskManager     task.TaskManager
	proxyTester     proxy.Tester
	subsManager     proxy.SubscriptionManager
	proxyService    proxy.ProxyService
	ipDetectService service.IPDetectorService
	jobIDs          map[string]cron.EntryID // 存储任务ID，用于更新

	configMutex   sync.RWMutex
	customConfigs map[uint]*model.SubscriptionConfig
	sysConfig     config.Config
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	return &Scheduler{
		cron:          cron.New(cron.WithSeconds(), cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger))),
		isRunning:     false,
		jobIDs:        make(map[string]cron.EntryID),
		customConfigs: make(map[uint]*model.SubscriptionConfig),
	}
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
}

// UpdateSubscriptionJob 更新订阅任务
func (s *Scheduler) UpdateSubscriptionJob(subID uint) error {
	s.configMutex.Lock()
	defer s.configMutex.Unlock()

	// 1. 获取最新配置
	subscriptionConfig, err := s.subsManager.GetSubscriptionConfig(subID)
	if err != nil {
		return err
	}

	// 2. 更新内存映射
	if subscriptionConfig != nil {
		s.customConfigs[subID] = subscriptionConfig
	} else {
		delete(s.customConfigs, subID)
	}

	// 3. 处理 Cron 任务
	jobName := "sub_update_" + strconv.FormatUint(uint64(subID), 10)

	// 先移除旧任务（如果存在）
	s.jobMutex.Lock()
	if entryID, exists := s.jobIDs[jobName]; exists {
		s.cron.Remove(entryID)
		delete(s.jobIDs, jobName)
		log.Infoln("Removed custom subscription update job %s", jobName)
	}
	s.jobMutex.Unlock()

	// 如果有自定义配置且开启了自动更新，添加新任务
	if subscriptionConfig != nil && subscriptionConfig.AutoUpdate && subscriptionConfig.UpdateInterval != "" {
		s.addCustomSubJob(subID, subscriptionConfig, s.sysConfig.Proxy)
	}
	return nil
}

// Init 启动调度器
func (s *Scheduler) Init(sysConfig config.Config) error {
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()

	// 如果已经在运行，先停止
	if s.isRunning {
		s.cron.Stop()
	}

	// 重新创建cron
	s.cron = cron.New(cron.WithSeconds(), cron.WithChain(cron.Recover(cron.DefaultLogger)))
	s.jobIDs = make(map[string]cron.EntryID)

	// 添加任务
	for _, job := range sysConfig.CronJobs {
		// 检查任务配置是否有效
		if job.Schedule == "" {
			log.Infoln("Job %s has invalid schedule, skipping", job.Name)
			continue
		}

		// 创建任务闭包
		jobConfig := job // 创建副本避免闭包问题
		entryID, err := s.cron.AddFunc(jobConfig.Schedule, func() {
			s.executeJob(jobConfig, sysConfig.IPCheck, sysConfig.Proxy)
		})

		if err != nil {
			log.Infoln("Failed to add job %s: %v", job.Name, err)
			continue
		}

		// 存储任务ID
		s.jobIDs[job.Name] = entryID
		log.Infoln("Added job %s with schedule %s", job.Name, job.Schedule)
	}

	// 处理订阅更新任务
	// 1. 获取所有订阅自定义配置
	customConfigs, err := s.subsManager.GetAllSubscriptionConfigs()
	if err != nil {
		log.Errorln("Failed to get subscription configs: %v", err)
	}

	s.configMutex.Lock()
	s.customConfigs = make(map[uint]*model.SubscriptionConfig)
	for _, cfg := range customConfigs {
		s.customConfigs[cfg.SubscriptionID] = cfg
	}
	s.configMutex.Unlock()

	// 2. 注册有个性化配置的任务
	s.configMutex.RLock()
	// 注意：这里我们只处理 Init 时的状态。UpdateSubscriptionJob 会处理运行时的变化。
	// 为了复用代码，UpdateSubscriptionJob 需要能够创建任务。
	// 但 Init 这里有 sysConfig 上下文。

	for subID, subCfg := range s.customConfigs {
		if subCfg.AutoUpdate && subCfg.UpdateInterval != "" {
			s.addCustomSubJob(subID, subCfg, sysConfig.Proxy)
		}
	}
	s.configMutex.RUnlock()

	// 3. 处理默认订阅更新任务（针对没有自定义配置的订阅）
	if sysConfig.DefaultSub.AutoUpdate && sysConfig.DefaultSub.Interval != "" {
		entryID, err := s.cron.AddFunc(sysConfig.DefaultSub.Interval, func() {
			ctx := context.Background()
			log.Infoln("Executing default subscription update job (filtered)")

			// 构造下载选项
			var opts *util.DownloadOptions
			if sysConfig.DefaultSub.UseProxy && sysConfig.Proxy.Enabled && sysConfig.Proxy.URL != "" {
				opts = &util.DownloadOptions{
					ProxyURL: sysConfig.Proxy.URL,
				}
			}

			// 找出所有需要按默认配置更新的订阅
			allSubs, _, err := s.subsManager.GetSubscriptionsPage(proxy.SubsPage{Page: 1, PageSize: 100000})
			if err != nil {
				log.Errorln("Failed to get all subscriptions for default update: %v", err)
				return
			}

			s.configMutex.RLock()
			defer s.configMutex.RUnlock()

			for _, sub := range allSubs {
				// 如果该订阅没有自定义配置，则由全局任务负责
				if _, hasCustom := s.customConfigs[sub.ID]; !hasCustom {
					if err := s.subsManager.RefreshSubscriptionAsync(ctx, sub.ID, opts); err != nil {
						log.Errorln("Default subscription update failed for sub %d: %v", sub.ID, err)
					}
				}
			}
		})

		if err != nil {
			log.Infoln("Failed to add default subscription update job: %v", err)
		} else {
			s.jobIDs["default_sub_update"] = entryID
			log.Infoln("Added filtered default subscription update job with schedule %s", sysConfig.DefaultSub.Interval)
		}
	}

	// 保存 sysConfig 以便后续使用 (需要修改 Struct)
	s.sysConfig = sysConfig

	// 启动cron
	s.cron.Start()
	s.isRunning = true

	return nil
}

// addCustomSubJob 辅助方法：添加自定义订阅任务
func (s *Scheduler) addCustomSubJob(subID uint, subCfg *model.SubscriptionConfig, proxyConfig config.Proxy) {
	jobName := "sub_update_" + strconv.FormatUint(uint64(subID), 10)

	// 使用闭包捕获
	entryID, err := s.cron.AddFunc(subCfg.UpdateInterval, func() {
		ctx := context.Background()
		log.Infoln("Executing custom subscription update job for sub %d", subID)

		var opts *util.DownloadOptions
		if subCfg.UseProxy && proxyConfig.Enabled && proxyConfig.URL != "" {
			opts = &util.DownloadOptions{
				ProxyURL: proxyConfig.URL,
			}
		}

		if err := s.subsManager.RefreshSubscriptionAsync(ctx, subID, opts); err != nil {
			log.Errorln("Custom subscription update failed for sub %d: %v", subID, err)
		}
	})

	if err != nil {
		log.Infoln("Failed to add custom job %s: %v", jobName, err)
	} else {
		s.jobMutex.Lock()
		s.jobIDs[jobName] = entryID
		s.jobMutex.Unlock()
		log.Infoln("Added custom subscription update job %s with schedule %s", jobName, subCfg.UpdateInterval)
	}
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()

	if s.isRunning {
		s.cron.Stop()
		s.isRunning = false
		log.Infoln("Scheduler stopped")
	}
}

// executeJob 执行定时任务
func (s *Scheduler) executeJob(job config.CronJob, checkConfig config.IPCheckConfig, proxyConfig config.Proxy) {
	log.Infoln("Executing job: %s", job.Name)

	defer func() {
		if r := recover(); r != nil {
			log.Infoln("Job %s panic: %v", job.Name, r)

			// 检查是否有正在运行的任务，尝试标记完成
			taskTypes := []task.TaskType{task.TaskTypeSpeedTest, task.TaskTypeReloadSubs}
			for _, taskType := range taskTypes {
				if s.taskManager.IsRunning(taskType) {
					s.taskManager.FinishTask(taskType, "任务执行过程中发生严重错误")
					log.Infoln("Forced task %s to finish due to panic", taskType)
				}
			}
		}
	}()

	// 通过这个方法来控制多个定时任务只能同时运行一个
	if s.taskManager.IsAnyRunning() {
		log.Infoln("Another task is running, skipping job: %s", job.Name)
		return
	}

	ctx := context.Background()

	// 步骤 1: 执行节点测试
	if job.TestProxy.Enable {
		// ... (此处省略后续 TestProxy, AutoBan 等逻辑，仅移除 ReloadSubscribeConfig 相关代码)
		log.Infoln("Job '%s': Start to test proxy.", job.Name)
		filter := &proxy.ProxyFilter{}
		if job.TestProxy.Status != "" {
			statusStrList := strings.Split(job.TestProxy.Status, ",")
			statusList := make([]model.ProxyStatus, len(statusStrList))
			for i, statusStr := range statusStrList {
				status, err := strconv.Atoi(statusStr)
				if err != nil {
					log.Errorln("Job '%s': Failed to convert status string to int: %v", job.Name, err)
					continue
				}
				statusList[i] = model.ProxyStatus(status)
			}
			if len(statusList) > 0 {
				filter.Status = statusList
			} else {
				filter = nil
			}
		} else {
			filter = nil
		}

		concurrent := job.TestProxy.Concurrent
		if concurrent == 0 {
			concurrent = 5
		}

		testRequest := &proxy.TestRequest{
			Filters:    filter,
			Concurrent: concurrent,
		}
		if err := s.proxyTester.TestProxies(ctx, testRequest, false); err != nil {
			log.Errorln("Job '%s': Failed to execute proxy testing: %v", job.Name, err)
		}
	}

	// 步骤3 禁用节点
	if job.AutoBan.Enable {
		log.Infoln("Job '%s': Start to ban proxy.", job.Name)
		testTimes := job.AutoBan.TestTimes
		if testTimes == 0 {
			testTimes = 5
		}
		serviceReq := proxy.BanProxyReq{
			SuccessRateThreshold:   job.AutoBan.SuccessRateThreshold,
			DownloadSpeedThreshold: job.AutoBan.DownloadSpeedThreshold,
			UploadSpeedThreshold:   job.AutoBan.UploadSpeedThreshold,
			PingThreshold:          job.AutoBan.PingThreshold,
			TestTimes:              testTimes,
		}
		if err := s.proxyService.BanProxy(ctx, serviceReq); err != nil {
			log.Errorln("Job '%s': Failed to ban proxy: %v", job.Name, err)
		}
	}

	if job.IPCheck.Enable {
		log.Infoln("Job '%s': Start to check ip quality.", job.Name)
		proxies, _, err := s.proxyService.GetProxiesByFilters(nil, "id", "asc", 1, 100000)
		if err != nil {
			log.Errorln("Job '%s': Failed to get proxies: %v", job.Name, err)
		}
		proxyIdList := make([]uint, 0, len(proxies))
		for _, singleProxy := range proxies {
			proxyIdList = append(proxyIdList, singleProxy.ID)
		}
		err = s.ipDetectService.BatchDetect(&service.BatchIPDetectorReq{
			ProxyIDList:     proxyIdList,
			Enabled:         true,
			IPInfoEnable:    job.IPCheck.IPInfo.Enable,
			APPUnlockEnable: job.IPCheck.AppUnlock.Enable,
			Refresh:         job.IPCheck.Refresh,
			Concurrent:      job.IPCheck.Concurrent,
		})
		if err != nil {
			log.Errorln("Job '%s': Failed to detect ip quality: %v", job.Name, err)
		}
	}

	if len(job.Webhook) > 0 {
		log.Infoln("Job '%s': Start to send webhook.", job.Name)
		webhookClient := util.NewWebhookClient()

		if errs := webhookClient.ExecuteWebhooks(job.Webhook, nil); len(errs) > 0 {
			for _, err := range errs {
				log.Errorln("Webhook execution error: %v", err)
			}
		} else {
			log.Infoln("Job '%s': All webhooks executed successfully", job.Name)
		}
	}

	log.Infoln("Job '%s' finished execution.", job.Name)
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
