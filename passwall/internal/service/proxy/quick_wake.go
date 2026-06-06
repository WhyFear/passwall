package proxy

import (
	"context"
	"errors"
	"fmt"

	"passwall/internal/adapter/speedtester"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"

	"github.com/metacubex/mihomo/log"
	"golang.org/x/sync/errgroup"
)

const defaultQuickWakeConcurrent = 50

type QuickWakeRequest struct {
	Types      []model.ProxyType
	Concurrent int
}

type QuickWakeService interface {
	WakeBannedProxies(ctx context.Context, request QuickWakeRequest, async bool) error
}

type quickWakeService struct {
	proxyRepo          repository.ProxyRepository
	speedTesterFactory speedtester.SpeedTesterFactory
	taskManager        task.TaskManager
}

func NewQuickWakeService(
	proxyRepo repository.ProxyRepository,
	speedTesterFactory speedtester.SpeedTesterFactory,
	taskManager task.TaskManager,
) QuickWakeService {
	return &quickWakeService{
		proxyRepo:          proxyRepo,
		speedTesterFactory: speedTesterFactory,
		taskManager:        taskManager,
	}
}

func (s *quickWakeService) WakeBannedProxies(ctx context.Context, request QuickWakeRequest, async bool) error {
	if ctx == nil {
		ctx = context.Background()
	}

	proxies, err := s.proxyRepo.FindByStatusAndTypesIncludingBanned(
		[]model.ProxyStatus{model.ProxyStatusBanned},
		request.Types,
	)
	if err != nil {
		return fmt.Errorf("查询待唤醒节点失败: %w", err)
	}
	if len(proxies) == 0 {
		log.Infoln("没有找到需要快速唤醒的节点")
		return nil
	}

	taskRun, started := task.StartRunWithSpec(ctx, s.taskManager, task.TaskSpec{
		Type:  task.TaskTypeQuickWake,
		Total: len(proxies),
		Accesses: []task.TaskAccess{
			{Resource: task.ResourceProxies, Mode: task.AccessModeWrite},
		},
	})
	if !started {
		return task.ErrTaskConflict
	}

	concurrent := request.Concurrent
	if concurrent <= 0 {
		concurrent = defaultQuickWakeConcurrent
	}

	if async {
		go s.runWake(taskRun, proxies, concurrent)
	} else {
		s.runWake(taskRun, proxies, concurrent)
	}
	return nil
}

func (s *quickWakeService) runWake(taskRun *task.TaskRun, proxies []*model.Proxy, concurrent int) {
	var finishMessage string
	defer func() {
		if r := recover(); r != nil {
			finishMessage = fmt.Sprintf("快速唤醒任务发生panic: %v", r)
			log.Errorln("%s", finishMessage)
		}
		taskRun.FinishWithContextMessage(finishMessage)
		log.Infoln("快速唤醒任务执行完毕")
	}()

	taskCtx := taskRun.Context()
	eg, groupCtx := errgroup.WithContext(taskCtx)
	eg.SetLimit(concurrent)

	for _, proxy := range proxies {
		if taskCtx.Err() != nil || groupCtx.Err() != nil {
			log.Infoln("快速唤醒任务已被取消，停止处理剩余节点")
			break
		}

		p := proxy
		eg.Go(func() error {
			defer taskRun.IncrementProgress("")
			if taskCtx.Err() != nil {
				return taskCtx.Err()
			}
			if s.isProxyAwake(taskCtx, p) && p.Status == model.ProxyStatusBanned {
				p.Status = model.ProxyStatusPending
				if err := s.proxyRepo.UpdateProxyStatus(p); err != nil {
					log.Errorln("更新快速唤醒节点状态失败[代理ID:%d]: %v", p.ID, err)
				}
			}
			return nil
		})
	}

	waitCh := make(chan struct{})
	go func() {
		_ = eg.Wait()
		close(waitCh)
	}()

	select {
	case <-taskCtx.Done():
		if errors.Is(taskCtx.Err(), context.Canceled) {
			finishMessage = task.TaskCanceledMessage
			log.Infoln("快速唤醒任务已被取消，等待正在进行的探测完成")
		} else {
			finishMessage = task.TaskTerminatedMessage
		}
		<-waitCh
	case <-waitCh:
		log.Infoln("快速唤醒任务全部探测完成")
	}
}

func (s *quickWakeService) isProxyAwake(ctx context.Context, proxy *model.Proxy) bool {
	tester, err := s.speedTesterFactory.GetSpeedTester(proxy.Type)
	if err != nil || tester == nil {
		log.Warnln("获取延迟探测器失败[代理ID:%d]: %v", proxy.ID, err)
		return false
	}
	result, err := tester.TestLatency(ctx, proxy)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			log.Infoln("快速唤醒探测已取消[代理ID:%d]", proxy.ID)
		} else {
			log.Warnln("快速唤醒探测失败[代理ID:%d]: %v", proxy.ID, err)
		}
		return false
	}
	if result == nil || result.Error != nil || result.Ping <= 0 {
		return false
	}
	return true
}
