package task

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

const (
	TaskCanceledMessage   = "任务被取消"
	TaskTerminatedMessage = "任务超时或其他原因终止"
)

// TaskRun wraps a global task lifecycle with progress accumulation and
// idempotent finishing. It keeps TaskManager's public interface unchanged.
type TaskRun struct {
	manager    TaskManager
	taskType   TaskType
	ctx        context.Context
	completed  atomic.Int32
	finishOnce sync.Once
}

func StartRun(ctx context.Context, manager TaskManager, taskType TaskType, total int) (*TaskRun, bool) {
	return StartRunWithSpec(ctx, manager, TaskSpec{
		Type:  taskType,
		Total: total,
	})
}

func StartRunWithSpec(ctx context.Context, manager TaskManager, spec TaskSpec) (*TaskRun, bool) {
	taskCtx, started := manager.StartTaskWithSpec(ctx, spec)
	if !started {
		return nil, false
	}
	return &TaskRun{
		manager:  manager,
		taskType: spec.Type,
		ctx:      taskCtx,
	}, true
}

func (r *TaskRun) Context() context.Context {
	return r.ctx
}

func (r *TaskRun) IncrementProgress(errMsg string) int {
	completed := int(r.completed.Add(1))
	r.manager.UpdateProgress(r.taskType, completed, errMsg)
	return completed
}

func (r *TaskRun) UpdateProgress(completed int, errMsg string) {
	r.completed.Store(int32(completed))
	r.manager.UpdateProgress(r.taskType, completed, errMsg)
}

func (r *TaskRun) Finish(errMsg string) {
	r.finishOnce.Do(func() {
		r.manager.FinishTask(r.taskType, errMsg)
	})
}

func (r *TaskRun) FinishWithContextMessage(errMsg string) {
	if errMsg == "" {
		errMsg = MessageForContext(r.ctx)
	}
	r.Finish(errMsg)
}

func MessageForContext(ctx context.Context) string {
	if ctx == nil || ctx.Err() == nil {
		return ""
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return TaskCanceledMessage
	}
	return TaskTerminatedMessage
}
