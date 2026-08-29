package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"passwall/config"
	"passwall/internal/model"
	proxyservice "passwall/internal/service/proxy"
	"passwall/internal/util"

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

func TestSchedulerInitRejectsInvalidSchedulesWithoutReplacingCurrentJobs(t *testing.T) {
	scheduler := NewScheduler()
	scheduler.SetServices(nil, nil, &fakeSubscriptionManager{}, nil, nil)
	require.NoError(t, scheduler.Init(config.Config{
		CronJobs: []config.CronJob{{Name: "current", Schedule: "0 0 0 1 1 *"}},
	}))
	defer scheduler.Stop()

	err := scheduler.Init(config.Config{
		CronJobs: []config.CronJob{
			{Name: "invalid", Schedule: "not a cron"},
		},
	})
	require.Error(t, err)

	status := scheduler.GetStatus()
	jobs := status["jobs"].(map[string]interface{})
	assert.Contains(t, jobs, "current")
	assert.NotContains(t, jobs, "invalid")
}

func TestSchedulerInitRejectsDuplicateAndReservedJobNames(t *testing.T) {
	scheduler := NewScheduler()
	scheduler.SetServices(nil, nil, &fakeSubscriptionManager{}, nil, nil)

	err := scheduler.Validate(config.Config{CronJobs: []config.CronJob{
		{Name: "duplicate", Schedule: "0 0 0 1 1 *"},
		{Name: "duplicate", Schedule: "0 0 1 1 1 *"},
	}})
	require.ErrorContains(t, err, "duplicate")

	err = scheduler.Validate(config.Config{CronJobs: []config.CronJob{
		{Name: "default_sub_update", Schedule: "0 0 0 1 1 *"},
	}})
	require.ErrorContains(t, err, "reserved")

	err = scheduler.Validate(config.Config{DefaultSub: config.DefaultSubscriptionUpdateConfig{AutoUpdate: true}})
	require.ErrorContains(t, err, "interval is empty")
}

func TestSchedulerInitRequiresServices(t *testing.T) {
	scheduler := NewScheduler()

	err := scheduler.Init(config.Config{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scheduler services are not set")
}

func TestSchedulerInitSkipsOrphanedSubscriptionConfigs(t *testing.T) {
	manager := &fakeSubscriptionManager{
		configs: []*model.SubscriptionConfig{{SubscriptionID: 7, AutoUpdate: true, UpdateInterval: "0 0 0 1 1 *"}},
		findErr: proxyservice.ErrSubscriptionNotFound,
	}
	scheduler := NewScheduler()
	scheduler.SetServices(nil, nil, manager, nil, nil)

	require.NoError(t, scheduler.Init(config.Config{}))
	defer scheduler.Stop()
	assert.NotContains(t, scheduler.GetStatus()["jobs"].(map[string]interface{}), "sub_update_7")
}

func TestSchedulerBeginStopClosesAdmission(t *testing.T) {
	scheduler := NewScheduler()
	scheduler.SetServices(nil, nil, &fakeSubscriptionManager{}, nil, nil)
	require.NoError(t, scheduler.Init(config.Config{}))

	stopContext := scheduler.BeginStop()

	select {
	case <-stopContext.Done():
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
	assert.ErrorContains(t, scheduler.UpdateSubscriptionJob(1), "stopped")
}

func TestUpdateSubscriptionJobKeepsOldJobWhenReplacementIsInvalid(t *testing.T) {
	manager := &fakeSubscriptionManager{sub: &model.Subscription{ID: 7, Status: model.SubscriptionStatusOK}}
	scheduler := NewScheduler()
	scheduler.SetServices(nil, nil, manager, nil, nil)
	require.NoError(t, scheduler.Init(config.Config{}))
	defer scheduler.Stop()

	manager.config = &model.SubscriptionConfig{SubscriptionID: 7, AutoUpdate: true, UpdateInterval: "0 0 0 1 1 *"}
	require.NoError(t, scheduler.UpdateSubscriptionJob(7))
	assert.Contains(t, scheduler.GetStatus()["jobs"].(map[string]interface{}), "sub_update_7")

	manager.config = &model.SubscriptionConfig{SubscriptionID: 7, AutoUpdate: true, UpdateInterval: "invalid"}
	require.Error(t, scheduler.UpdateSubscriptionJob(7))
	assert.Contains(t, scheduler.GetStatus()["jobs"].(map[string]interface{}), "sub_update_7")
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

func TestDefaultSubscriptionJobUsesKeysetBatches(t *testing.T) {
	subscriptions := make([]*model.Subscription, schedulerBatchSize+1)
	for index := range subscriptions {
		subscriptions[index] = &model.Subscription{ID: uint(index + 1)}
	}
	manager := &fakeSubscriptionManager{subscriptions: subscriptions}
	scheduler := NewScheduler()
	scheduler.SetServices(nil, nil, manager, nil, nil)
	require.NoError(t, scheduler.Init(config.Config{DefaultSub: config.DefaultSubscriptionUpdateConfig{
		AutoUpdate: true,
		Interval:   "0 0 0 1 1 *",
	}}))
	defer scheduler.Stop()

	scheduler.cron.Entry(scheduler.jobIDs["default_sub_update"]).WrappedJob.Run()

	assert.Equal(t, []subscriptionCursorCall{{0, schedulerBatchSize}, {schedulerBatchSize, schedulerBatchSize}}, manager.cursorCalls)
	require.Len(t, manager.refreshedIDs, schedulerBatchSize+1)
	assert.Equal(t, uint(schedulerBatchSize+1), manager.refreshedIDs[schedulerBatchSize])
}

type subscriptionCursorCall struct {
	afterID uint
	limit   int
}

type fakeSubscriptionManager struct {
	proxyservice.SubscriptionManager
	sub           *model.Subscription
	config        *model.SubscriptionConfig
	configs       []*model.SubscriptionConfig
	findErr       error
	subscriptions []*model.Subscription
	cursorCalls   []subscriptionCursorCall
	refreshedIDs  []uint
}

func (f *fakeSubscriptionManager) GetAllSubscriptionConfigs() ([]*model.SubscriptionConfig, error) {
	return f.configs, nil
}

func (f *fakeSubscriptionManager) GetSubscriptionByID(uint) (*model.Subscription, error) {
	return f.sub, f.findErr
}

func (f *fakeSubscriptionManager) GetSubscriptionConfig(uint) (*model.SubscriptionConfig, error) {
	return f.config, nil
}

func (f *fakeSubscriptionManager) GetSubscriptionsAfterID(afterID uint, limit int) ([]*model.Subscription, error) {
	f.cursorCalls = append(f.cursorCalls, subscriptionCursorCall{afterID, limit})
	result := make([]*model.Subscription, 0, limit)
	for _, subscription := range f.subscriptions {
		if subscription.ID > afterID {
			result = append(result, subscription)
			if len(result) == limit {
				break
			}
		}
	}
	return result, nil
}

func (f *fakeSubscriptionManager) RefreshSubscriptionAsync(_ context.Context, id uint, _ *util.DownloadOptions) error {
	f.refreshedIDs = append(f.refreshedIDs, id)
	return nil
}
