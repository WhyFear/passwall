package scheduler

import (
	"context"
	"github.com/metacubex/mihomo/log"
	"passwall/internal/service/proxy"
	"passwall/internal/service/task"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"passwall/config"
	"passwall/internal/service"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	cron        *cron.Cron
	jobMutex    sync.Mutex
	isRunning   bool
	taskManager task.TaskManager
	proxyTester service.ProxyTester
	subsManager proxy.SubscriptionManager
	jobIDs      map[string]cron.EntryID // 存储任务ID，用于更新
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
func (s *Scheduler) SetServices(taskManager task.TaskManager, proxyTester service.ProxyTester, subsManager proxy.SubscriptionManager) {
	s.taskManager = taskManager
	s.proxyTester = proxyTester
	s.subsManager = subsManager
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

	// 添加panic恢复机制
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
	shouldTest := job.TestAll || job.TestNew || job.TestFailed || job.TestSpeed
	if shouldTest {
		// 创建测试请求，但禁用其中的订阅刷新功能，因为我们已经在上一步处理过了
		request := &service.TestProxyRequest{
			ReloadSubscribeConfig: false, // IMPORTANT: Already handled above
			TestAll:               job.TestAll,
			TestNew:               job.TestNew,
			TestFailed:            job.TestFailed,
			TestSpeed:             job.TestSpeed,
			Concurrent:            job.Concurrent,
		}

		if err := s.proxyTester.TestProxies(request); err != nil {
			log.Errorln("Job '%s': Failed to execute proxy testing: %v", job.Name, err)
		}
	} else {
		log.Infoln("Job '%s': No proxy testing configured, skipping proxy test.", job.Name)
	}

	// 未来可以在这里添加其他步骤...

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
