package proxy

import (
	"context"
	"testing"

	"passwall/config"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteSubscriptionCancelsItsRefresh(t *testing.T) {
	taskManager := task.NewTaskManager()
	taskCtx, started := taskManager.StartResourceTask(context.Background(), task.TaskTypeReloadSubs, 7, 1)
	require.True(t, started)
	go func() {
		<-taskCtx.Done()
		taskManager.FinishResourceTask(task.TaskTypeReloadSubs, 7, task.TaskCanceledMessage)
	}()

	repo := &fakeDeleteSubscriptionRepository{}
	manager := &subscriptionManagerImpl{
		subscriptionRepo: repo,
		refresher:        &subscriptionRefresher{taskManager: taskManager},
	}

	require.NoError(t, manager.DeleteSubscription(7))
	assert.Equal(t, uint(7), repo.deletedID)
	assert.False(t, taskManager.IsResourceRunning(task.TaskTypeReloadSubs, 7))
}

func TestRefreshSubscriptionRejectsDeletedSubscription(t *testing.T) {
	manager := &subscriptionManagerImpl{
		subscriptionRepo: &fakeDeleteSubscriptionRepository{subscription: &model.Subscription{
			ID: 7, Status: model.SubscriptionStatusDeleted,
		}},
	}

	err := manager.RefreshSubscriptionAsync(context.Background(), 7, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "订阅已删除")
}

func TestSaveSubscriptionConfigRejectsMissingSubscription(t *testing.T) {
	configs := &fakeSubscriptionConfigRepository{}
	manager := &subscriptionManagerImpl{
		subscriptionRepo:       &fakeDeleteSubscriptionRepository{},
		subscriptionConfigRepo: configs,
		configProvider:         &fakeConfigProvider{cfg: &config.Config{}},
	}

	err := manager.SaveSubscriptionConfig(&model.SubscriptionConfig{
		SubscriptionID: 7,
		AutoUpdate:     true,
		UpdateInterval: "0 0 0 * * *",
	})

	require.Error(t, err)
	assert.Nil(t, configs.saved)
}

func TestSaveSubscriptionConfigRejectsInvalidCron(t *testing.T) {
	configs := &fakeSubscriptionConfigRepository{}
	manager := &subscriptionManagerImpl{
		subscriptionRepo: &fakeDeleteSubscriptionRepository{subscription: &model.Subscription{
			ID: 7, Status: model.SubscriptionStatusOK,
		}},
		subscriptionConfigRepo: configs,
		configProvider:         &fakeConfigProvider{cfg: &config.Config{}},
	}

	err := manager.SaveSubscriptionConfig(&model.SubscriptionConfig{
		SubscriptionID: 7,
		AutoUpdate:     true,
		UpdateInterval: "invalid",
	})

	require.Error(t, err)
	assert.Nil(t, configs.saved)
}

func TestSaveSubscriptionConfigRollsBackWhenSchedulerUpdateFails(t *testing.T) {
	oldConfig := &model.SubscriptionConfig{SubscriptionID: 7, AutoUpdate: false}
	configs := &fakeSubscriptionConfigRepository{current: oldConfig}
	manager := &subscriptionManagerImpl{
		subscriptionRepo: &fakeDeleteSubscriptionRepository{subscription: &model.Subscription{
			ID: 7, Status: model.SubscriptionStatusOK,
		}},
		subscriptionConfigRepo: configs,
		configProvider:         &fakeConfigProvider{cfg: &config.Config{}},
		scheduler:              &fakeSubscriptionScheduler{err: assert.AnError},
	}

	err := manager.SaveSubscriptionConfig(&model.SubscriptionConfig{
		SubscriptionID: 7,
		AutoUpdate:     true,
		UpdateInterval: "0 0 0 * * *",
	})

	require.Error(t, err)
	assert.Same(t, oldConfig, configs.current)
}

type fakeDeleteSubscriptionRepository struct {
	repository.SubscriptionRepository
	subscription *model.Subscription
	deletedID    uint
}

type fakeSubscriptionConfigRepository struct {
	repository.SubscriptionConfigRepository
	current *model.SubscriptionConfig
	saved   *model.SubscriptionConfig
}

func (r *fakeSubscriptionConfigRepository) FindByID(uint) (*model.SubscriptionConfig, error) {
	return r.current, nil
}

func (r *fakeSubscriptionConfigRepository) Save(config *model.SubscriptionConfig) error {
	r.saved = config
	r.current = config
	return nil
}

func (r *fakeSubscriptionConfigRepository) Delete(uint) error {
	r.current = nil
	return nil
}

type fakeSubscriptionScheduler struct {
	err error
}

func (s *fakeSubscriptionScheduler) UpdateSubscriptionJob(uint) error {
	return s.err
}

func (r *fakeDeleteSubscriptionRepository) FindByID(uint) (*model.Subscription, error) {
	return r.subscription, nil
}

func (r *fakeDeleteSubscriptionRepository) Delete(id uint) error {
	r.deletedID = id
	return nil
}
