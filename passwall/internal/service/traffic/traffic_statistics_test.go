package traffic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"passwall/config"
	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatisticsServiceStopsPeriodicProcessingPromptly(t *testing.T) {
	service := NewTrafficStatisticsService(nil)

	for range 2 {
		require.NoError(t, service.Restart(config.ClashAPIConfig{Enable: true}))
		started := time.Now()
		require.NoError(t, service.Stop())
		require.Less(t, time.Since(started), time.Second)
	}
}

func TestProcessTrafficDeltaUsesOnlyLeafStableProxyID(t *testing.T) {
	service := NewTrafficStatisticsService(nil)
	started := time.Now()
	generation := newTrafficGeneration(nil, started, 1)
	connectionStart := started.Add(time.Second)

	service.processTrafficDelta(generation, 0, Connections{Connections: []Connection{
		{ID: "leaf", Upload: 10, Download: 20, Start: connectionStart, Chains: []string{"[pw:42]-same", "auto", "select"}},
		{ID: "same-name-other-id", Upload: 3, Download: 4, Start: connectionStart, Chains: []string{"[pw:43]-same", "auto"}},
		{ID: "builtin", Upload: 100, Download: 200, Start: connectionStart, Chains: []string{"DIRECT", "domestic"}},
		{ID: "empty", Upload: 100, Download: 200, Start: connectionStart},
	}})

	assert.Equal(t, trafficDelta{Upload: 10, Download: 20}, generation.pending[42])
	assert.Equal(t, trafficDelta{Upload: 3, Download: 4}, generation.pending[43])
	assert.Len(t, generation.pending, 2)
	assert.Len(t, generation.lastValues[0], 4, "ignored exits still need snapshots")
}

func TestFlushTrafficRequeuesFailedBatch(t *testing.T) {
	repo := &fakeIncrementTrafficRepository{failures: 1}
	service := NewTrafficStatisticsService(repo)
	generation := newTrafficGeneration(nil, time.Now(), 0)
	generation.pending[42] = trafficDelta{Upload: 10, Download: 20}

	require.Error(t, service.flushTrafficToDB(generation))
	assert.Equal(t, trafficDelta{Upload: 10, Download: 20}, generation.pending[42])

	require.NoError(t, service.flushTrafficToDB(generation))
	assert.Empty(t, generation.pending)
	require.Len(t, repo.accepted, 1)
	assert.Equal(t, model.TrafficStatistics{ProxyID: 42, UploadTotal: 10, DownloadTotal: 20}, repo.accepted[0][0])
}

func TestStopRetriesFailedFinalFlushWithoutStartingAnotherGeneration(t *testing.T) {
	repo := &fakeIncrementTrafficRepository{failures: 1}
	service := NewTrafficStatisticsService(repo)
	require.NoError(t, service.Restart(config.ClashAPIConfig{Enable: true}))
	firstGeneration := service.run
	firstGeneration.mu.Lock()
	firstGeneration.pending[42] = trafficDelta{Upload: 10, Download: 20}
	firstGeneration.mu.Unlock()

	require.Error(t, service.Stop())
	assert.Same(t, firstGeneration, service.run, "failed final flush must remain retryable")
	require.NoError(t, service.Stop())
	assert.Nil(t, service.run)
	require.NoError(t, service.Stop(), "Stop must be idempotent")
	assert.Len(t, repo.accepted, 1)
}

func TestRestartReplacesAndWaitsForPreviousGeneration(t *testing.T) {
	service := NewTrafficStatisticsService(nil)
	require.NoError(t, service.Restart(config.ClashAPIConfig{Enable: true}))
	firstGeneration := service.run

	require.NoError(t, service.Restart(config.ClashAPIConfig{Enable: true}))
	assert.NotSame(t, firstGeneration, service.run)
	assert.ErrorIs(t, firstGeneration.ctx.Err(), context.Canceled)
	require.NoError(t, service.Stop())
}

func TestRestartRejectsTrafficSchemaWithoutProxyIDUniqueIndex(t *testing.T) {
	repo := &fakeIncrementTrafficRepository{validateErr: errors.New("missing unique index")}
	service := NewTrafficStatisticsService(repo)

	err := service.Restart(config.ClashAPIConfig{Enable: true, Clients: []config.ClashAPIClient{{URL: "ws://127.0.0.1"}}})

	require.ErrorContains(t, err, "missing unique index")
	assert.Nil(t, service.run)
}

func TestDialClientErrorsDoNotExposeClientCredentials(t *testing.T) {
	service := NewTrafficStatisticsService(nil)
	service.maxRetries = 1
	for _, client := range []config.ClashAPIClient{
		{URL: "ws://user:url-password@%41?token=url-secret", Secret: "clash-secret"},
		{URL: "http://user:url-password@127.0.0.1?token=url-secret", Secret: "clash-secret"},
	} {
		_, err := service.dialClient(context.Background(), 7, client, service.maxRetries)
		require.Error(t, err)
		for _, secret := range []string{"url-password", "url-secret", "clash-secret"} {
			assert.NotContains(t, err.Error(), secret)
		}
	}
}

func TestRestartDoesNotRetryInitialConnection(t *testing.T) {
	service := NewTrafficStatisticsService(&fakeIncrementTrafficRepository{})
	service.baseRetryInterval = time.Second
	service.maxRetryInterval = time.Second
	service.maxRetries = 3
	started := time.Now()
	err := service.Restart(config.ClashAPIConfig{Enable: true, Clients: []config.ClashAPIClient{{
		URL: "ws://127.0.0.1:1",
	}}})

	require.Error(t, err)
	assert.Less(t, time.Since(started), 500*time.Millisecond)
}

type fakeIncrementTrafficRepository struct {
	repository.TrafficRepository
	mu          sync.Mutex
	failures    int
	validateErr error
	accepted    [][]model.TrafficStatistics
}

func (r *fakeIncrementTrafficRepository) IncrementTraffic(batch []model.TrafficStatistics) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failures > 0 {
		r.failures--
		return errors.New("database unavailable")
	}
	copy := append([]model.TrafficStatistics(nil), batch...)
	r.accepted = append(r.accepted, copy)
	return nil
}

func (r *fakeIncrementTrafficRepository) ValidateProxyIDUnique() error { return r.validateErr }
