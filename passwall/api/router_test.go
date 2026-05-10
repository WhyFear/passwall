package api

import (
	"testing"

	"passwall/config"
	"passwall/internal/service"

	"github.com/stretchr/testify/require"
)

func TestSetupRouterRegistersWebProxyRoutes(t *testing.T) {
	require.NotPanics(t, func() {
		_ = SetupRouter(&config.Config{Token: "token"}, &service.Services{}, nil)
	})
}
