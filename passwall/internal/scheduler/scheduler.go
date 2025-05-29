package scheduler

import (
	"log"
	"sync"

	"github.com/robfig/cron/v3"

	"passwall/config"
	"passwall/internal/service"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	cron        *cron.Cron
	jobMutex    sync.Mutex
	isRunning   bool
	taskManager service.TaskManager
	proxyTester service.ProxyTester
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	return &Scheduler{
		cron:      cron.New(cron.WithSeconds()),
		isRunning: false,
	}
}

// SetServices 设置服务
func (s *Scheduler) SetServices(taskManager service.TaskManager, proxyTester service.ProxyTester) {
	s.taskManager = taskManager
	s.proxyTester = proxyTester
}

// Start 启动调度器
func (s *Scheduler) Start(cronJobs []config.CronJob) error {
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()

	// 如果已经在运行，先停止
	if s.isRunning {
		s.cron.Stop()
	}

	// 重新创建cron
	s.cron = cron.New(cron.WithSeconds())

	// 添加任务
	for _, job := range cronJobs {
		// 创建任务闭包
		jobConfig := job // 创建副本避免闭包问题
		_, err := s.cron.AddFunc(jobConfig.Schedule, func() {
			s.executeJob(jobConfig)
		})

		if err != nil {
			log.Printf("Failed to add job %s: %v", job.Name, err)
			continue
		}

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
	}
}

// executeJob 执行定时任务
func (s *Scheduler) executeJob(job config.CronJob) {
	log.Printf("Executing job: %s", job.Name)

	// 检查是否有任务在运行
	if s.taskManager.IsAnyTaskRunning() {
		log.Printf("Another task is running, skipping job: %s", job.Name)
		return
	}

	// 创建测试请求
	request := &service.TestProxyRequest{
		ReloadSubscribeConfig: job.ReloadSubscribeConfig,
		TestAll:               job.TestAll,
		TestFailed:            job.TestFailed,
		TestSpeed:             job.TestSpeed,
		Concurrent:            job.Concurrent,
	}

	// 执行测试
	if err := s.proxyTester.TestProxies(request); err != nil {
		log.Printf("Failed to execute job %s: %v", job.Name, err)
	}
}
