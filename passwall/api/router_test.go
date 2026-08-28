package api

import (
	"net/http"
	"net/http/httptest"
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

func TestSubscribeRouteRejectsQueryToken(t *testing.T) {
	router := SetupRouter(&config.Config{Token: "secret"}, &service.Services{}, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/subscribe?token=secret", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusUnauthorized, response.Code)
}
