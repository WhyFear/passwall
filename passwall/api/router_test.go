package api

import (
	"bytes"
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

func TestProtectedRoutesAuthenticateBeforeReadingBody(t *testing.T) {
	router := SetupRouter(&config.Config{Token: "secret"}, &service.Services{}, nil)
	body := &countingReader{Reader: bytes.NewReader(make([]byte, 1<<20))}
	request := httptest.NewRequest(http.MethodPost, "/web/api/config", body)
	request.ContentLength = -1
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Zero(t, body.reads)
}

type countingReader struct {
	*bytes.Reader
	reads int
}

func (r *countingReader) Read(p []byte) (int, error) {
	r.reads++
	return r.Reader.Read(p)
}
