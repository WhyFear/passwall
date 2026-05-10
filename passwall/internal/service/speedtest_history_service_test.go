package service

import (
	"testing"
	"time"

	"passwall/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpeedTestHistoryServiceSuccessRatesUseLatestSummaries(t *testing.T) {
	repo := &fakeSuccessRateHistoryRepo{
		summaries: map[uint][]repository.SpeedTestHistorySummary{
			1: {
				{ProxyID: 1, DownloadSpeed: 0, CreatedAt: time.Now()},
				{ProxyID: 1, DownloadSpeed: 100, CreatedAt: time.Now().Add(-time.Minute)},
				{ProxyID: 1, DownloadSpeed: 200, CreatedAt: time.Now().Add(-2 * time.Minute)},
				{ProxyID: 1, DownloadSpeed: 0, CreatedAt: time.Now().Add(-3 * time.Minute)},
				{ProxyID: 1, DownloadSpeed: 300, CreatedAt: time.Now().Add(-4 * time.Minute)},
			},
			2: {
				{ProxyID: 2, DownloadSpeed: 0},
				{ProxyID: 2, DownloadSpeed: 0},
			},
		},
	}
	service := NewSpeedTestHistoryService(repo)

	rates, err := service.GetSuccessRatesByProxyIDList([]uint{1, 2, 3}, 5)

	require.NoError(t, err)
	assert.Equal(t, []uint{1, 2, 3}, repo.proxyIDs)
	assert.Equal(t, 5, repo.limit)
	assert.Equal(t, 60.0, rates[1])
	assert.Equal(t, 0.0, rates[2])
	_, ok := rates[3]
	assert.False(t, ok)
}

func TestSpeedTestHistoryServiceSuccessRatesEmptyInputDoesNotQuery(t *testing.T) {
	repo := &fakeSuccessRateHistoryRepo{}
	service := NewSpeedTestHistoryService(repo)

	rates, err := service.GetSuccessRatesByProxyIDList(nil, 5)

	require.NoError(t, err)
	assert.Empty(t, rates)
	assert.False(t, repo.called)
}

type fakeSuccessRateHistoryRepo struct {
	repository.SpeedTestHistoryRepository
	summaries map[uint][]repository.SpeedTestHistorySummary
	proxyIDs  []uint
	limit     int
	called    bool
}

func (f *fakeSuccessRateHistoryRepo) BatchFindLatestSummariesByProxyIDList(proxyIDList []uint, limit int) (map[uint][]repository.SpeedTestHistorySummary, error) {
	f.called = true
	f.proxyIDs = proxyIDList
	f.limit = limit
	return f.summaries, nil
}

var _ repository.SpeedTestHistoryRepository = (*fakeSuccessRateHistoryRepo)(nil)
