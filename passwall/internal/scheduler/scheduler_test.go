package scheduler

import (
	"sync/atomic"
	"testing"
	"time"

	"passwall/config"
	"passwall/internal/model"
	proxyservice "passwall/internal/service/proxy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchedulerInitRegistersConfiguredJobs(t *testing.T) {
	scheduler := NewScheduler()
	scheduler.SetServices(nil, nil, &fakeSubscriptionManager{}, nil, nil)

	err := scheduler.Init(config.Config{
		CronJobs: []config.CronJob{
			{Name: "nightly", Schedule: "0 0 0 1 1 *"},
		},
	})
	require.NoError(t, err)
	defer scheduler.Stop()

	status := scheduler.GetStatus()
	assert.Equal(t, true, status["is_running"])
	jobs := status["jobs"].(map[string]interface{})
	assert.Contains(t, jobs, "nightly")
}

func TestSchedulerInitSkipsInvalidSchedules(t *testing.T) {
	scheduler := NewScheduler()
	scheduler.SetServices(nil, nil, &fakeSubscriptionManager{}, nil, nil)

	err := scheduler.Init(config.Config{
		CronJobs: []config.CronJob{
			{Name: "invalid", Schedule: "not a cron"},
		},
	})
	require.NoError(t, err)
	defer scheduler.Stop()

	status := scheduler.GetStatus()
	jobs := status["jobs"].(map[string]interface{})
	assert.NotContains(t, jobs, "invalid")
}

func TestSchedulerInitRequiresServices(t *testing.T) {
	scheduler := NewScheduler()

	err := scheduler.Init(config.Config{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scheduler services are not set")
}

func TestSchedulerCronSkipsOverlappingRuns(t *testing.T) {
	cronScheduler := newCron()
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	var calls atomic.Int32

	entryID, err := cronScheduler.AddFunc("0 0 0 1 1 *", func() {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
	})
	require.NoError(t, err)
	job := cronScheduler.Entry(entryID).WrappedJob
	go job.Run()
	<-started

	secondDone := make(chan struct{})
	go func() {
		job.Run()
		close(secondDone)
	}()

	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("overlapping cron run was not skipped")
	}
	assert.Equal(t, int32(1), calls.Load())
}

func TestSchedulerCronContinuesAfterRecoveredPanic(t *testing.T) {
	cronScheduler := newCron()
	var calls atomic.Int32

	entryID, err := cronScheduler.AddFunc("0 0 0 1 1 *", func() {
		if calls.Add(1) == 1 {
			panic("boom")
		}
	})
	require.NoError(t, err)
	job := cronScheduler.Entry(entryID).WrappedJob

	job.Run()
	job.Run()

	assert.Equal(t, int32(2), calls.Load())
}

type fakeSubscriptionManager struct {
	proxyservice.SubscriptionManager
}

func (f *fakeSubscriptionManager) GetAllSubscriptionConfigs() ([]*model.SubscriptionConfig, error) {
	return nil, nil
}
