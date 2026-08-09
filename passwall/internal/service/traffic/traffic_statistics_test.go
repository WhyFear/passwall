package traffic

import (
	"testing"
	"time"

	"passwall/config"

	"github.com/stretchr/testify/require"
)

func TestStatisticsServiceStopsPeriodicProcessingPromptly(t *testing.T) {
	service := NewTrafficStatisticsService(enabledEmptyClashConfig{}, nil, nil)

	for range 2 {
		require.NoError(t, service.Start())
		started := time.Now()
		service.Stop()
		require.Less(t, time.Since(started), time.Second)
	}
}

type enabledEmptyClashConfig struct{}

func (enabledEmptyClashConfig) GetClashClients() ([]config.ClashAPIClient, bool) {
	return nil, true
}
