package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"passwall/internal/service/task"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIPDetectorBatchDetectTracksProgress(t *testing.T) {
	taskManager := task.NewTaskManager()
	var detected []uint
	var mu sync.Mutex
	detectorService := ipDetectorImpl{
		TaskManager: taskManager,
		detectOne: func(req *IPDetectorReq) error {
			mu.Lock()
			defer mu.Unlock()
			detected = append(detected, req.ProxyID)
			return nil
		},
	}

	err := detectorService.BatchDetect(&BatchIPDetectorReq{
		ProxyIDList:     []uint{1, 2, 3},
		Enabled:         true,
		IPInfoEnable:    true,
		APPUnlockEnable: true,
		Refresh:         true,
		Concurrent:      2,
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []uint{1, 2, 3}, detected)
	status := taskManager.GetStatus(task.TaskTypeCheckIp)
	require.NotNil(t, status)
	assert.Equal(t, task.TaskStateFinished, status.State)
	assert.Equal(t, 3, status.Completed)
	assert.Equal(t, 100, status.Progress)
	assert.Equal(t, "batch detect proxy ip finished", status.Error)
}

func TestIPDetectorBatchDetectCancellationStopsPendingDetects(t *testing.T) {
	taskManager := task.NewTaskManager()
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	var calls atomic.Int32
	detectorService := ipDetectorImpl{
		TaskManager: taskManager,
		detectOne: func(req *IPDetectorReq) error {
			if calls.Add(1) == 1 {
				close(started)
				<-release
			}
			return nil
		},
	}

	go func() {
		done <- detectorService.BatchDetect(&BatchIPDetectorReq{
			ProxyIDList: []uint{1, 2, 3},
			Enabled:     true,
			Concurrent:  1,
		})
	}()

	require.Eventually(t, func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	cancelled, timedOut := taskManager.CancelTask(task.TaskTypeCheckIp, false)
	require.True(t, cancelled)
	require.False(t, timedOut)
	close(release)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("batch detect did not finish after cancellation")
	}

	status := taskManager.GetStatus(task.TaskTypeCheckIp)
	require.NotNil(t, status)
	assert.Equal(t, task.TaskStateFinished, status.State)
	assert.Equal(t, task.TaskCanceledMessage, status.Error)
	assert.Equal(t, int32(1), calls.Load())
}

func TestIPDetectorBatchDetectUsesDefaultConcurrency(t *testing.T) {
	taskManager := task.NewTaskManager()
	detectorService := ipDetectorImpl{
		TaskManager: taskManager,
		detectOne: func(req *IPDetectorReq) error {
			return nil
		},
	}
	req := &BatchIPDetectorReq{
		ProxyIDList: []uint{1},
		Enabled:     true,
	}

	err := detectorService.BatchDetect(req)

	require.NoError(t, err)
	assert.Equal(t, 20, req.Concurrent)
}
