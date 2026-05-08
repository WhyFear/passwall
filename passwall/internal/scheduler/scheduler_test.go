package scheduler

import (
	"testing"

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

type fakeSubscriptionManager struct {
	proxyservice.SubscriptionManager
}

func (f *fakeSubscriptionManager) GetAllSubscriptionConfigs() ([]*model.SubscriptionConfig, error) {
	return nil, nil
}
