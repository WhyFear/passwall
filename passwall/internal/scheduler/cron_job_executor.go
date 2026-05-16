package scheduler

import (
	"context"
	"strconv"
	"strings"

	"passwall/config"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service"
	"passwall/internal/service/proxy"
	"passwall/internal/service/task"
	"passwall/internal/util"

	"github.com/metacubex/mihomo/log"
)

type cronJobExecutor struct {
	taskManager     task.TaskManager
	proxyTester     proxy.Tester
	proxyService    proxy.ProxyService
	ipDetectService service.IPDetectorService
	webhookClient   *util.WebhookClient
}

func newCronJobExecutor(
	taskManager task.TaskManager,
	proxyTester proxy.Tester,
	proxyService proxy.ProxyService,
	ipDetectService service.IPDetectorService,
) *cronJobExecutor {
	return &cronJobExecutor{
		taskManager:     taskManager,
		proxyTester:     proxyTester,
		proxyService:    proxyService,
		ipDetectService: ipDetectService,
		webhookClient:   util.NewWebhookClient(),
	}
}

func (e *cronJobExecutor) Execute(job config.CronJob) {
	log.Infoln("Executing job: %s", job.Name)
	defer e.recoverJob(job)

	ctx := context.Background()
	e.executeProxyTest(ctx, job)
	e.executeAutoBan(ctx, job)
	e.executeIPCheck(job)
	e.executeWebhooks(job)
	log.Infoln("Job '%s' finished execution.", job.Name)
}

func (e *cronJobExecutor) recoverJob(job config.CronJob) {
	if r := recover(); r == nil {
		return
	} else {
		log.Infoln("Job %s panic: %v", job.Name, r)
	}

	for _, status := range e.taskManager.GetAllStatus() {
		if status.State != task.TaskStateRunning && status.State != task.TaskStateCanceling {
			continue
		}
		if status.ResourceID == 0 {
			e.taskManager.FinishTask(status.Type, "任务执行过程中发生严重错误")
		} else {
			e.taskManager.FinishResourceTask(status.Type, status.ResourceID, "任务执行过程中发生严重错误")
		}
		log.Infoln("Forced task %s (resource=%d) to finish due to panic", status.Type, status.ResourceID)
	}
}

func (e *cronJobExecutor) executeProxyTest(ctx context.Context, job config.CronJob) {
	if !job.TestProxy.Enable {
		return
	}
	log.Infoln("Job '%s': Start to test proxy.", job.Name)
	testRequest := &proxy.TestRequest{
		Filters:    buildProxyFilter(job.TestProxy.Status),
		Concurrent: withDefault(job.TestProxy.Concurrent, 5),
	}
	if err := e.proxyTester.TestProxies(ctx, testRequest, false); err != nil {
		log.Errorln("Job '%s': Failed to execute proxy testing: %v", job.Name, err)
	}
}

func (e *cronJobExecutor) executeAutoBan(ctx context.Context, job config.CronJob) {
	if !job.AutoBan.Enable {
		return
	}
	log.Infoln("Job '%s': Start to ban proxy.", job.Name)
	serviceReq := proxy.BanProxyReq{
		SuccessRateThreshold:   job.AutoBan.SuccessRateThreshold,
		DownloadSpeedThreshold: job.AutoBan.DownloadSpeedThreshold,
		UploadSpeedThreshold:   job.AutoBan.UploadSpeedThreshold,
		PingThreshold:          job.AutoBan.PingThreshold,
		TestTimes:              withDefault(job.AutoBan.TestTimes, 5),
	}
	if err := e.proxyService.BanProxy(ctx, serviceReq); err != nil {
		log.Errorln("Job '%s': Failed to ban proxy: %v", job.Name, err)
	}
}

func (e *cronJobExecutor) executeIPCheck(job config.CronJob) {
	if !job.IPCheck.Enable {
		return
	}
	log.Infoln("Job '%s': Start to check ip quality.", job.Name)
	proxies, _, err := e.proxyService.GetProxiesByFilters(nil, "id", "asc", 1, 100000)
	if err != nil {
		log.Errorln("Job '%s': Failed to get proxies: %v", job.Name, err)
		return
	}
	proxyIDList := make([]uint, 0, len(proxies))
	for _, singleProxy := range proxies {
		proxyIDList = append(proxyIDList, singleProxy.ID)
	}
	err = e.ipDetectService.BatchDetect(context.Background(), &service.BatchIPDetectorReq{
		ProxyIDList:     proxyIDList,
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

func (e *cronJobExecutor) executeWebhooks(job config.CronJob) {
	if len(job.Webhook) == 0 {
		return
	}
	log.Infoln("Job '%s': Start to send webhook.", job.Name)
	if errs := e.webhookClient.ExecuteWebhooks(job.Webhook, nil); len(errs) > 0 {
		for _, err := range errs {
			log.Errorln("Webhook execution error: %v", err)
		}
		return
	}
	log.Infoln("Job '%s': All webhooks executed successfully", job.Name)
}

func buildProxyFilter(statusText string) *repository.NodeFilter {
	if statusText == "" {
		return nil
	}
	statusStrList := strings.Split(statusText, ",")
	statusList := make([]model.ProxyStatus, 0, len(statusStrList))
	for _, statusStr := range statusStrList {
		status, err := strconv.Atoi(statusStr)
		if err != nil {
			log.Errorln("Failed to convert status string to int: %v", err)
			continue
		}
		statusList = append(statusList, model.ProxyStatus(status))
	}
	if len(statusList) == 0 {
		return nil
	}
	return &repository.NodeFilter{Status: statusList}
}

func withDefault(value int, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}
