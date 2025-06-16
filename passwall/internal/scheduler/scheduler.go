package scheduler

import (
	"github.com/enfein/mieru/v3/pkg/log"
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
	cron         *cron.Cron
	jobMutex     sync.Mutex
	isRunning    bool
	taskManager  task.TaskManager
	proxyTester  service.ProxyTester
	proxyService proxy.ProxyService
	jobIDs       map[string]cron.EntryID // 存储任务ID，用于更新
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
func (s *Scheduler) SetServices(taskManager task.TaskManager, proxyTester service.ProxyTester, proxyService proxy.ProxyService) {
	s.taskManager = taskManager
	s.proxyTester = proxyTester
	s.proxyService = proxyService
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
			log.Printf("Job %s has invalid schedule, skipping", job.Name)
			continue
		}

		// 创建任务闭包
		jobConfig := job // 创建副本避免闭包问题
		entryID, err := s.cron.AddFunc(jobConfig.Schedule, func() {
			s.executeJob(jobConfig)
		})

		if err != nil {
			log.Printf("Failed to add job %s: %v", job.Name, err)
			continue
		}

		// 存储任务ID
		s.jobIDs[job.Name] = entryID
		log.Printf("Added job %s with schedule %s", job.Name, job.Schedule)
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
		log.Println("Scheduler stopped")
	}
}

// UpdateJob 更新单个任务
func (s *Scheduler) UpdateJob(job config.CronJob) error {
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()

	// 如果任务已存在，先移除
	if id, exists := s.jobIDs[job.Name]; exists {
		s.cron.Remove(id)
		delete(s.jobIDs, job.Name)
	}

	// 添加新任务
	jobConfig := job // 创建副本避免闭包问题
	entryID, err := s.cron.AddFunc(jobConfig.Schedule, func() {
		s.executeJob(jobConfig)
	})

	if err != nil {
		log.Printf("Failed to update job %s: %v", job.Name, err)
		return err
	}

	// 存储任务ID
	s.jobIDs[job.Name] = entryID
	log.Printf("Updated job %s with schedule %s", job.Name, job.Schedule)
	return nil
}

// executeJob 执行定时任务
func (s *Scheduler) executeJob(job config.CronJob) {
	log.Printf("Executing job: %s", job.Name)

	// 添加panic恢复机制
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Job %s panic: %v", job.Name, r)

			// 检查是否有正在运行的任务，尝试标记完成
			taskTypes := []task.TaskType{task.TaskTypeSpeedTest, task.TaskTypeReloadSubs}
			for _, taskType := range taskTypes {
				if s.taskManager.IsRunning(taskType) {
					s.taskManager.FinishTask(taskType, "任务执行过程中发生严重错误")
					log.Printf("Forced task %s to finish due to panic", taskType)
				}
			}
		}
	}()

	// 检查是否有任务在运行
	if s.taskManager.IsAnyRunning() {
		log.Printf("Another task is running, skipping job: %s", job.Name)
		return
	}

	// 创建测试请求
	request := &service.TestProxyRequest{
		ReloadSubscribeConfig: job.ReloadSubscribeConfig,
		TestAll:               job.TestAll,
		TestNew:               job.TestNew,
		TestFailed:            job.TestFailed,
		TestSpeed:             job.TestSpeed,
		Concurrent:            job.Concurrent,
	}

	// 执行测试
	if err := s.proxyTester.TestProxies(request); err != nil {
		log.Printf("Failed to execute job %s: %v", job.Name, err)
	}
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
