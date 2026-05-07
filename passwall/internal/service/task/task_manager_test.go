package task

import (
	"context"
	"testing"

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
}
