package proxy

import (
	"context"
	"fmt"
	"strings"

	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"
	"passwall/internal/util"

	"github.com/metacubex/mihomo/log"
)

type downloadSubscriptionFunc func(context.Context, string, *util.DownloadOptions) ([]byte, error)

type subscriptionRefresher struct {
	subscriptionRepo repository.SubscriptionRepository
	taskManager      task.TaskManager
	configProvider   SystemConfigProvider
	proxyTester      Tester
	proxySyncer      *proxySyncer
	download         downloadSubscriptionFunc
}

func newSubscriptionRefresher(
	subscriptionRepo repository.SubscriptionRepository,
	taskManager task.TaskManager,
	configProvider SystemConfigProvider,
	proxyTester Tester,
	proxySyncer *proxySyncer,
	download downloadSubscriptionFunc,
) *subscriptionRefresher {
	return &subscriptionRefresher{
		subscriptionRepo: subscriptionRepo,
		taskManager:      taskManager,
		configProvider:   configProvider,
		proxyTester:      proxyTester,
		proxySyncer:      proxySyncer,
		download:         download,
	}
}

func (r *subscriptionRefresher) RefreshAsync(ctx context.Context, subscription *model.Subscription, options *util.DownloadOptions) {
	taskType := task.TaskTypeReloadSubs
	taskCtx := ctx
	taskRun, started := task.StartRun(ctx, r.taskManager, taskType, 1)
	if started {
		taskCtx = taskRun.Context()
	}

	go func() {
		finishMessage := ""
		if started {
			defer func() {
				taskRun.FinishWithContextMessage(finishMessage)
			}()
		}

		err := r.RefreshOne(taskCtx, subscription, options)
		if !started {
			return
		}
		if err != nil {
			if taskCtx.Err() != nil {
				finishMessage = task.MessageForContext(taskCtx)
				return
			}
			log.Errorln("刷新订阅失败: %v", err)
			finishMessage = err.Error()
			taskRun.UpdateProgress(1, finishMessage)
			return
		}
		taskRun.UpdateProgress(1, "")
	}()
}

func (r *subscriptionRefresher) RefreshMany(ctx context.Context, subscriptions []*model.Subscription, options *util.DownloadOptions, async bool) {
	if len(subscriptions) == 0 {
		log.Infoln("没有找到订阅配置")
		return
	}

	taskType := task.TaskTypeReloadSubs
	taskCtx := ctx
	taskRun, started := task.StartRun(ctx, r.taskManager, taskType, len(subscriptions))
	if started {
		taskCtx = taskRun.Context()
	}

	run := func() {
		r.refreshMany(taskCtx, taskRun, subscriptions, options)
	}
	if async {
		go run()
		return
	}
	run()
}

func (r *subscriptionRefresher) refreshMany(ctx context.Context, taskRun *task.TaskRun, subscriptions []*model.Subscription, options *util.DownloadOptions) {
	var finishMessage string

	defer func() {
		if taskRun == nil {
			return
		}
		if recoverValue := recover(); recoverValue != nil {
			finishMessage = fmt.Sprintf("刷新订阅任务发生panic: %v", recoverValue)
			log.Errorln("%s", finishMessage)
		}
		taskRun.FinishWithContextMessage(finishMessage)
	}()

	var lastError error
	completed := 0
	stoppedByContext := false
	jobsTotal := len(subscriptions)
	jobsDone := 0

subscriptionLoop:
	for _, subscription := range subscriptions {
		select {
		case <-ctx.Done():
			if !stoppedByContext {
				stoppedByContext = true
				log.Infoln("任务已被取消，停止处理剩余订阅")
			}
			break subscriptionLoop
		default:
		}

		if err := r.RefreshOne(ctx, subscription, options); err != nil {
			if ctx.Err() != nil {
				stoppedByContext = true
				lastError = ctx.Err()
				break subscriptionLoop
			}
			log.Errorln("刷新订阅[%s]失败: %v", subscription.URL, err)
			lastError = err
		}

		jobsDone++
		completed++
		if taskRun != nil {
			taskRun.UpdateProgress(completed, "")
		}
	}

	if stoppedByContext {
		finishMessage = task.MessageForContext(ctx)
	} else if lastError != nil {
		finishMessage = lastError.Error()
	}

	if taskRun != nil {
		taskRun.FinishWithContextMessage(finishMessage)
	}

	log.Infoln("所有订阅刷新完成, 共处理 %d 个订阅, 完成 %d 个, 错误: %v", jobsTotal, jobsDone, lastError)
}

func (r *subscriptionRefresher) RefreshOne(ctx context.Context, subscription *model.Subscription, options *util.DownloadOptions) error {
	taskType := task.TaskTypeReloadSubs
	taskCtx, started := r.taskManager.StartTaskWithSpec(ctx, task.TaskSpec{
		Type:       taskType,
		ResourceID: subscription.ID,
		Total:      1,
		Accesses: []task.TaskAccess{
			{Resource: task.ResourceSubscriptions, ResourceID: subscription.ID, Mode: task.AccessModeWrite},
			{Resource: task.ResourceProxies, ResourceID: subscription.ID, Mode: task.AccessModeWrite},
		},
	})
	if !started {
		log.Infoln("订阅[ID:%d]正在刷新中，本次跳过", subscription.ID)
		return fmt.Errorf("订阅[ID:%d]正在刷新或存在冲突任务", subscription.ID)
	}

	var err error
	triggerPendingTest := false
	defer func() {
		if triggerPendingTest {
			r.triggerPendingProxyTest(taskCtx)
		}
	}()
	defer func() {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		r.taskManager.FinishResourceTask(taskType, subscription.ID, errMsg)
	}()

	log.Infoln("开始刷新订阅: %s", subscription.URL)
	if subscription.URL == "" {
		err = fmt.Errorf("订阅为空")
		return err
	}
	if !strings.HasPrefix(subscription.URL, "http") {
		log.Infoln("非下载链接，无需刷新")
		return nil
	}

	downloadOptions := buildDownloadOptions(options)
	if downloadOptions.ProxyURL != "" {
		log.Infoln("使用代理下载: %s", downloadOptions.ProxyURL)
	}

	var content []byte
	content, err = r.download(taskCtx, subscription.URL, downloadOptions)
	if err != nil {
		if taskCtx.Err() != nil {
			return taskCtx.Err()
		}
		log.Errorln("下载订阅内容失败: %v", err)
		_ = markSubscriptionInvalid(r.subscriptionRepo, subscription)
		return fmt.Errorf("下载订阅内容失败: %w", err)
	}

	result, err := r.proxySyncer.Sync(taskCtx, subscription, content)
	if err != nil {
		if taskCtx.Err() != nil {
			return taskCtx.Err()
		}
		log.Errorln("解析订阅内容失败: %v", err)
		_ = markSubscriptionInvalid(r.subscriptionRepo, subscription)
		return err
	}

	if err = markSubscriptionOK(r.subscriptionRepo, subscription, content); err != nil {
		return err
	}
	logProxySyncResult(subscription, result)

	triggerPendingTest = true
	r.taskManager.UpdateResourceProgress(taskType, subscription.ID, 1, "")
	return nil
}

func buildDownloadOptions(options *util.DownloadOptions) *util.DownloadOptions {
	downloadOptions := &util.DownloadOptions{
		Timeout:     util.DefaultDownloadOptions.Timeout,
		MaxFileSize: util.DefaultDownloadOptions.MaxFileSize,
	}
	if options == nil {
		return downloadOptions
	}
	downloadOptions.ProxyURL = options.ProxyURL
	if options.Timeout > 0 {
		downloadOptions.Timeout = options.Timeout
	}
	if options.MaxFileSize > 0 {
		downloadOptions.MaxFileSize = options.MaxFileSize
	}
	return downloadOptions
}

func (r *subscriptionRefresher) triggerPendingProxyTest(ctx context.Context) {
	if r.proxyTester == nil {
		return
	}
	if ctx != nil && ctx.Err() != nil {
		return
	}

	log.Infoln("开始自动测试新获取的代理节点...")
	concurrent := 5
	if cfg, err := r.configProvider.GetConfig(); err == nil && cfg.Concurrent > 0 {
		concurrent = cfg.Concurrent
	}

	testReq := &TestRequest{
		Filters: &ProxyFilter{
			Status: []model.ProxyStatus{model.ProxyStatusPending},
		},
		Concurrent: concurrent,
	}
	go func() {
		if err := r.proxyTester.TestProxies(ctx, testReq, false); err != nil {
			log.Errorln("自动测试代理失败: %v", err)
		}
	}()
}
