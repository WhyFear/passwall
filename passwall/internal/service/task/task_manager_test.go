package task

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskManagerLifecycle(t *testing.T) {
	manager := NewTaskManager()

	ctx, started := manager.StartTask(context.Background(), TaskTypeSpeedTest, 4)
	require.True(t, started)
	require.NotNil(t, ctx)
	assert.True(t, manager.IsRunning(TaskTypeSpeedTest))
	assert.True(t, manager.IsAnyRunning())

	_, duplicate := manager.StartTask(context.Background(), TaskTypeSpeedTest, 4)
	assert.False(t, duplicate)

	manager.UpdateProgress(TaskTypeSpeedTest, 2, "")
	status := manager.GetStatus(TaskTypeSpeedTest)
	require.NotNil(t, status)
	assert.Equal(t, 50, status.Progress)
	assert.Equal(t, 2, status.Completed)

	manager.FinishTask(TaskTypeSpeedTest, "done")
	status = manager.GetStatus(TaskTypeSpeedTest)
	require.NotNil(t, status)
	assert.Equal(t, TaskStateFinished, status.State)
	assert.Equal(t, 100, status.Progress)
	assert.Equal(t, "done", status.Error)
	assert.False(t, manager.IsRunning(TaskTypeSpeedTest))
	assert.False(t, manager.IsAnyRunning())
}

func TestTaskManagerResourceTasksAreIndependent(t *testing.T) {
	manager := NewTaskManager()

	_, startedOne := manager.StartResourceTask(context.Background(), TaskTypeReloadSubs, 1, 1)
	_, startedTwo := manager.StartResourceTask(context.Background(), TaskTypeReloadSubs, 2, 1)
	_, duplicateOne := manager.StartResourceTask(context.Background(), TaskTypeReloadSubs, 1, 1)

	assert.True(t, startedOne)
	assert.True(t, startedTwo)
	assert.False(t, duplicateOne)
	assert.True(t, manager.IsResourceRunning(TaskTypeReloadSubs, 1))
	assert.True(t, manager.IsResourceRunning(TaskTypeReloadSubs, 2))

	manager.FinishResourceTask(TaskTypeReloadSubs, 1, "")
	assert.False(t, manager.IsResourceRunning(TaskTypeReloadSubs, 1))
	assert.True(t, manager.IsResourceRunning(TaskTypeReloadSubs, 2))
}

func TestTaskManagerCancelTask(t *testing.T) {
	manager := NewTaskManager()

	ctx, started := manager.StartTask(context.Background(), TaskTypeCheckIp, 1)
	require.True(t, started)

	cancelled, timedOut := manager.CancelTask(TaskTypeCheckIp, false)

	assert.True(t, cancelled)
	assert.False(t, timedOut)
	assert.ErrorIs(t, ctx.Err(), context.Canceled)
	assert.True(t, manager.IsRunning(TaskTypeCheckIp))
	status := manager.GetStatus(TaskTypeCheckIp)
	require.NotNil(t, status)
	assert.Equal(t, TaskStateCanceling, status.State)

	_, duplicate := manager.StartTask(context.Background(), TaskTypeCheckIp, 1)
	assert.False(t, duplicate)

	manager.FinishTask(TaskTypeCheckIp, TaskCanceledMessage)
	assert.False(t, manager.IsRunning(TaskTypeCheckIp))
	_, restarted := manager.StartTask(context.Background(), TaskTypeCheckIp, 1)
	assert.True(t, restarted)
}

func TestTaskManagerCancelTaskTimeoutKeepsTaskCanceling(t *testing.T) {
	manager := NewTaskManager()
	originalTimeout := cancelWaitTimeout
	cancelWaitTimeout = 10 * time.Millisecond
	t.Cleanup(func() {
		cancelWaitTimeout = originalTimeout
	})

	_, started := manager.StartTask(context.Background(), TaskTypeSpeedTest, 1)
	require.True(t, started)

	cancelled, timedOut := manager.CancelTask(TaskTypeSpeedTest, true)

	assert.True(t, cancelled)
	assert.True(t, timedOut)
	assert.True(t, manager.IsRunning(TaskTypeSpeedTest))
	status := manager.GetStatus(TaskTypeSpeedTest)
	require.NotNil(t, status)
	assert.Equal(t, TaskStateCanceling, status.State)
	assert.Contains(t, status.Error, "仍在清理中")

	_, duplicate := manager.StartTask(context.Background(), TaskTypeSpeedTest, 1)
	assert.False(t, duplicate)

	manager.FinishTask(TaskTypeSpeedTest, TaskCanceledMessage)
	assert.False(t, manager.IsRunning(TaskTypeSpeedTest))
}

func TestTaskRunAccumulatesProgressAndFinishesOnce(t *testing.T) {
	manager := NewTaskManager()
	run, started := StartRun(context.Background(), manager, TaskTypeCheckIp, 3)
	require.True(t, started)

	run.IncrementProgress("")
	run.IncrementProgress("")

	status := manager.GetStatus(TaskTypeCheckIp)
	require.NotNil(t, status)
	assert.Equal(t, 2, status.Completed)
	assert.Equal(t, 66, status.Progress)

	run.Finish("first")
	run.Finish("second")

	status = manager.GetStatus(TaskTypeCheckIp)
	require.NotNil(t, status)
	assert.Equal(t, TaskStateFinished, status.State)
	assert.Equal(t, "first", status.Error)
}

func TestTaskRunFinishWithContextMessageUsesCancellationMessage(t *testing.T) {
	manager := NewTaskManager()
	ctx, cancel := context.WithCancel(context.Background())
	run, started := StartRun(ctx, manager, TaskTypeCheckIp, 1)
	require.True(t, started)

	cancel()
	run.FinishWithContextMessage("")

	status := manager.GetStatus(TaskTypeCheckIp)
	require.NotNil(t, status)
	assert.Equal(t, TaskCanceledMessage, status.Error)
}
