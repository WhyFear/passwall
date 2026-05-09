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

func TestTaskManagerAllowsConcurrentReadsOnSameResource(t *testing.T) {
	manager := NewTaskManager()

	_, startedOne := manager.StartTaskWithSpec(context.Background(), TaskSpec{
		Type:  TaskTypeCheckIp,
		Total: 1,
		Accesses: []TaskAccess{{
			Resource: ResourceProxies,
			Mode:     AccessModeRead,
		}},
	})
	_, startedTwo := manager.StartTaskWithSpec(context.Background(), TaskSpec{
		Type:       TaskTypeReloadSubs,
		ResourceID: 1,
		Total:      1,
		Accesses: []TaskAccess{{
			Resource: ResourceProxies,
			Mode:     AccessModeRead,
		}},
	})

	assert.True(t, startedOne)
	assert.True(t, startedTwo)
}

func TestTaskManagerRejectsReadWriteConflictOnSameResource(t *testing.T) {
	manager := NewTaskManager()

	_, startedWrite := manager.StartTaskWithSpec(context.Background(), TaskSpec{
		Type:  TaskTypeSpeedTest,
		Total: 1,
		Accesses: []TaskAccess{{
			Resource: ResourceProxies,
			Mode:     AccessModeWrite,
		}},
	})
	_, startedRead := manager.StartTaskWithSpec(context.Background(), TaskSpec{
		Type:       TaskTypeReloadSubs,
		ResourceID: 1,
		Total:      1,
		Accesses: []TaskAccess{{
			Resource: ResourceProxies,
			Mode:     AccessModeRead,
		}},
	})

	assert.True(t, startedWrite)
	assert.False(t, startedRead)
}

func TestTaskManagerAllowsWritesOnDifferentResourceIDs(t *testing.T) {
	manager := NewTaskManager()

	_, startedOne := manager.StartTaskWithSpec(context.Background(), TaskSpec{
		Type:       TaskTypeReloadSubs,
		ResourceID: 1,
		Total:      1,
		Accesses: []TaskAccess{{
			Resource:   ResourceProxies,
			ResourceID: 1,
			Mode:       AccessModeWrite,
		}},
	})
	_, startedTwo := manager.StartTaskWithSpec(context.Background(), TaskSpec{
		Type:       TaskTypeReloadSubs,
		ResourceID: 2,
		Total:      1,
		Accesses: []TaskAccess{{
			Resource:   ResourceProxies,
			ResourceID: 2,
			Mode:       AccessModeWrite,
		}},
	})
	_, startedGlobalWrite := manager.StartTaskWithSpec(context.Background(), TaskSpec{
		Type:  TaskTypeBanProxy,
		Total: 1,
		Accesses: []TaskAccess{{
			Resource: ResourceProxies,
			Mode:     AccessModeWrite,
		}},
	})

	assert.True(t, startedOne)
	assert.True(t, startedTwo)
	assert.False(t, startedGlobalWrite)
}

func TestTaskManagerAggregatesResourceStatusForTaskType(t *testing.T) {
	manager := NewTaskManager()

	_, startedOne := manager.StartResourceTask(context.Background(), TaskTypeReloadSubs, 1, 2)
	_, startedTwo := manager.StartResourceTask(context.Background(), TaskTypeReloadSubs, 2, 2)
	require.True(t, startedOne)
	require.True(t, startedTwo)

	manager.UpdateResourceProgress(TaskTypeReloadSubs, 1, 1, "")
	manager.UpdateResourceProgress(TaskTypeReloadSubs, 2, 2, "")

	status := manager.GetStatus(TaskTypeReloadSubs)
	require.NotNil(t, status)
	assert.Equal(t, TaskStateRunning, status.State)
	assert.Equal(t, 4, status.Total)
	assert.Equal(t, 3, status.Completed)
	assert.Equal(t, 75, status.Progress)
}

func TestTaskManagerCancelTaskCancelsActiveResourceTasks(t *testing.T) {
	manager := NewTaskManager()

	ctxOne, startedOne := manager.StartResourceTask(context.Background(), TaskTypeReloadSubs, 1, 1)
	ctxTwo, startedTwo := manager.StartResourceTask(context.Background(), TaskTypeReloadSubs, 2, 1)
	require.True(t, startedOne)
	require.True(t, startedTwo)

	cancelled, timedOut := manager.CancelTask(TaskTypeReloadSubs, false)

	assert.True(t, cancelled)
	assert.False(t, timedOut)
	assert.ErrorIs(t, ctxOne.Err(), context.Canceled)
	assert.ErrorIs(t, ctxTwo.Err(), context.Canceled)
	status := manager.GetStatus(TaskTypeReloadSubs)
	require.NotNil(t, status)
	assert.Equal(t, TaskStateCanceling, status.State)
	assert.Equal(t, 2, status.Total)
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
	manager := NewTaskManagerWithTimeout(10 * time.Millisecond)

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

func TestTaskRunWithResourceIDUpdatesResourceProgressAndFinish(t *testing.T) {
	manager := NewTaskManager()
	run, started := StartRunWithSpec(context.Background(), manager, TaskSpec{
		Type:       TaskTypeCheckIp,
		ResourceID: 42,
		Total:      5,
	})
	require.True(t, started)

	run.IncrementProgress("")
	run.IncrementProgress("")

	status := manager.GetStatus(TaskTypeCheckIp)
	require.NotNil(t, status)
	assert.Equal(t, 2, status.Completed)
	assert.Equal(t, 40, status.Progress)

	run.Finish("resource done")
	run.Finish("should not overwrite")

	assert.False(t, manager.IsResourceRunning(TaskTypeCheckIp, 42))
	allStatus := manager.GetAllStatus()
	var finishedStatus *TaskStatus
	for _, s := range allStatus {
		if s.Type == TaskTypeCheckIp && s.ResourceID == 42 {
			finishedStatus = s
			break
		}
	}
	require.NotNil(t, finishedStatus)
	assert.Equal(t, TaskStateFinished, finishedStatus.State)
	assert.Equal(t, "resource done", finishedStatus.Error)
}

func TestTaskRunDifferentResourceIDsCompleteIndependently(t *testing.T) {
	manager := NewTaskManager()

	run1, started1 := StartRunWithSpec(context.Background(), manager, TaskSpec{
		Type:       TaskTypeCheckIp,
		ResourceID: 1,
		Total:      1,
	})
	run2, started2 := StartRunWithSpec(context.Background(), manager, TaskSpec{
		Type:       TaskTypeCheckIp,
		ResourceID: 2,
		Total:      1,
	})
	require.True(t, started1)
	require.True(t, started2)

	run1.Finish("done 1")
	assert.False(t, manager.IsResourceRunning(TaskTypeCheckIp, 1))
	assert.True(t, manager.IsResourceRunning(TaskTypeCheckIp, 2))

	run2.Finish("done 2")
	assert.False(t, manager.IsResourceRunning(TaskTypeCheckIp, 2))
}
