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
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	return &Scheduler{
		cron:      cron.New(cron.WithSeconds(), cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger))),
		isRunning: false,
		jobIDs:    make(map[string]cron.EntryID),
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

// Init 启动调度器
func (s *Scheduler) Init(cronJobs []config.CronJob) error {
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
	for _, job := range cronJobs {
		// 检查任务配置是否有效
		if job.Schedule == "" {
			log.Infoln("Job %s has invalid schedule, skipping", job.Name)
			continue
		}

		// 创建任务闭包
		jobConfig := job // 创建副本避免闭包问题
		entryID, err := s.cron.AddFunc(jobConfig.Schedule, func() {
			s.executeJob(jobConfig)
		})

		if err != nil {
			log.Infoln("Failed to add job %s: %v", job.Name, err)
			continue
		}

		// 存储任务ID
		s.jobIDs[job.Name] = entryID
		log.Infoln("Added job %s with schedule %s", job.Name, job.Schedule)
	}

	// 启动cron
	s.cron.Start()
	s.isRunning = true

	return nil
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
func (s *Scheduler) executeJob(job config.CronJob) {
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

	// 步骤 1: 如果配置了刷新订阅，则串行执行
	if job.ReloadSubscribeConfig {
		if err := s.subsManager.RefreshAllSubscriptions(ctx, false); err != nil {
			log.Infoln("Job '%s': Failed to refresh subscriptions, stopping job. Error: %v", job.Name, err)
			return // 如果刷新失败，则终止当前任务
		}
		log.Infoln("Job '%s': Subscription refresh finished.", job.Name)
	}

	// 步骤 2: 执行节点测试
	if job.TestProxy.Enable {
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
		err := s.proxyService.BanProxy(ctx, serviceReq)
		if err != nil {
			log.Errorln("Job '%s': Failed to ban proxy: %v", job.Name, err)
		}
	}

	if job.IPCheck.Enable {
		filters := make(map[string]interface{})
		filters["status"] = model.ProxyStatusOK
		proxies, _, err := s.proxyService.GetProxiesByFilters(filters, "id", "asc", 1, 100000)
		if err != nil {
			log.Errorln("Job '%s': Failed to get proxies: %v", job.Name, err)
		}
		proxyIdList := make([]uint, len(proxies))
		for _, singleProxy := range proxies {
			proxyIdList = append(proxyIdList, singleProxy.ID)
		}
		err = s.ipDetectService.BatchDetect(&service.BatchIPDetectorReq{
			ProxyIDList:     proxyIdList,
			Enabled:         true,
			IPInfoEnable:    job.IPCheck.IPInfo.Enable,
			APPUnlockEnable: job.IPCheck.AppUnlock.Enable,
			Refresh:         job.IPCheck.Refresh,
		})
		if err != nil {
			log.Errorln("Job '%s': Failed to detect ip quality: %v", job.Name, err)
		}
	}

	if len(job.Webhook) > 0 {
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
