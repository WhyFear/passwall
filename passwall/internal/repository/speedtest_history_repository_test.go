package repository

import (
	"testing"
	"time"

	"passwall/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSpeedTestHistoryRepositoryBatchFindLatestSummariesUsesLimitAndOrder(t *testing.T) {
	db := newSpeedTestHistoryRepositoryTestDB(t)
	repo := NewSpeedTestHistoryRepository(db)
	now := time.Now()

	for i := 0; i < 7; i++ {
		require.NoError(t, db.Create(&model.SpeedTestHistory{
			ProxyID:       1,
			DownloadSpeed: i + 1,
			CreatedAt:     now.Add(-time.Duration(i) * time.Minute),
			TestTime:      now.Add(-time.Duration(i) * time.Minute),
		}).Error)
	}
	require.NoError(t, db.Create(&model.SpeedTestHistory{
		ProxyID:       2,
		DownloadSpeed: 99,
		CreatedAt:     now,
		TestTime:      now,
	}).Error)

	result, err := repo.BatchFindLatestSummariesByProxyIDList([]uint{1, 2, 3}, 5)

	require.NoError(t, err)
	require.Len(t, result[1], 5)
	assert.Equal(t, 1, result[1][0].DownloadSpeed)
	assert.Equal(t, 5, result[1][4].DownloadSpeed)
	require.Len(t, result[2], 1)
	assert.Equal(t, 99, result[2][0].DownloadSpeed)
	assert.NotContains(t, result, uint(3))
}

func TestSpeedTestHistoryRepositoryBatchFindLatestSummariesEmptyInput(t *testing.T) {
	db := newSpeedTestHistoryRepositoryTestDB(t)
	repo := NewSpeedTestHistoryRepository(db)

	result, err := repo.BatchFindLatestSummariesByProxyIDList(nil, 5)

	require.NoError(t, err)
	assert.Empty(t, result)
}

func newSpeedTestHistoryRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SpeedTestHistory{}))
	return db
}
