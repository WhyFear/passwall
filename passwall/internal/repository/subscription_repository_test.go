package repository

import (
	"testing"

	"passwall/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSubscriptionRepositoryDeleteRemovesConfigAndPreventsStatusRevival(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Subscription{}, &model.SubscriptionConfig{}))

	subscription := &model.Subscription{
		URL: "https://example.test/sub", Content: "old", Type: model.SubscriptionTypeClash, Status: model.SubscriptionStatusOK,
	}
	require.NoError(t, db.Create(subscription).Error)
	require.NoError(t, db.Create(&model.SubscriptionConfig{
		SubscriptionID: subscription.ID, AutoUpdate: true, UpdateInterval: "0 0 * * * *",
	}).Error)

	repo := NewSubscriptionRepository(db)
	require.NoError(t, repo.Delete(subscription.ID))

	var configCount int64
	require.NoError(t, db.Model(&model.SubscriptionConfig{}).Where("subscription_id = ?", subscription.ID).Count(&configCount).Error)
	assert.Zero(t, configCount)

	subscription.Status = model.SubscriptionStatusOK
	subscription.Content = "late"
	require.NoError(t, repo.UpdateStatusAndContent(subscription))
	subscription.Status = model.SubscriptionStatusInvalid
	require.NoError(t, repo.UpdateStatus(subscription))

	stored, err := repo.FindByID(subscription.ID)
	require.NoError(t, err)
	assert.Equal(t, model.SubscriptionStatusDeleted, stored.Status)
	assert.Equal(t, "old", stored.Content)
}

func TestSubscriptionRepositoryFindAfterIDUsesKeysetAndSkipsDeleted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Subscription{}))
	subscriptions := []*model.Subscription{
		{URL: "one", Type: model.SubscriptionTypeClash, Status: model.SubscriptionStatusOK},
		{URL: "two", Type: model.SubscriptionTypeClash, Status: model.SubscriptionStatusDeleted},
		{URL: "three", Type: model.SubscriptionTypeClash, Status: model.SubscriptionStatusOK},
	}
	require.NoError(t, db.Create(&subscriptions).Error)

	items, err := NewSubscriptionRepository(db).FindAfterID(subscriptions[0].ID, 1)

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, subscriptions[2].ID, items[0].ID)
}
