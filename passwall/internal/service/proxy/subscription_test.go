package proxy

import (
	"context"
	"testing"

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

type fakeDeleteSubscriptionRepository struct {
	repository.SubscriptionRepository
	subscription *model.Subscription
	deletedID    uint
}

func (r *fakeDeleteSubscriptionRepository) FindByID(uint) (*model.Subscription, error) {
	return r.subscription, nil
}

func (r *fakeDeleteSubscriptionRepository) Delete(id uint) error {
	r.deletedID = id
	return nil
}
