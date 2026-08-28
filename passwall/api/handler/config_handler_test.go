package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"passwall/config"
	"passwall/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfigOmitsCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Token:      "management-secret",
		Concurrent: 5,
		Server:     config.Server{Address: "0.0.0.0:8080"},
		Database:   config.Database{Driver: "postgres", DSN: "postgres://db-secret"},
		Proxy:      config.Proxy{Enabled: true, URL: "http://proxy-user:proxy-secret@proxy.example"},
		IPCheck: config.IPCheckConfig{IPInfo: config.IPInfoConfig{
			Enable: true,
			Scamalytics: config.Scamalytics{
				Host:   "https://scamalytics-secret.example",
				User:   "scamalytics-user-secret",
				APIKey: "scamalytics-api-secret",
			},
		}},
		ClashAPI: config.ClashAPIConfig{Enable: true, Clients: []config.ClashAPIClient{{
			URL: "ws://clash.example?token=clash-url-secret", Secret: "clash-secret",
		}}},
		CronJobs: []config.CronJob{{
			Name: "notify",
			Webhook: []config.WebhookConfig{{
				Name: "hook", Method: http.MethodPost,
				URL: "https://hook.example?token=hook-url-secret", Header: "authorization-secret", Body: "body-secret",
			}},
		}},
	}
	router := gin.New()
	router.GET("/config", NewConfigHandler(&fakeConfigHandlerService{cfg: cfg}).GetConfig)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config", nil))

	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	for _, secret := range []string{
		"management-secret", "0.0.0.0:8080", "postgres", "db-secret",
		"proxy.example", "proxy-secret", "scamalytics-secret", "scamalytics-user-secret", "scamalytics-api-secret",
		"clash.example", "clash-url-secret", "clash-secret", "hook.example", "hook-url-secret", "authorization-secret", "body-secret",
	} {
		assert.NotContains(t, body, secret)
	}
	assert.Contains(t, body, `"url_configured":true`)
	assert.Contains(t, body, `"secret_configured":true`)
	assert.Contains(t, body, `"existing_index":0`)
	assert.Contains(t, body, `"cron_jobs":[{"existing_index":0`)
	assert.Contains(t, body, `"webhook":[{"existing_index":0`)
	assert.Contains(t, body, `"configured":true`)
	assert.False(t, strings.Contains(body, `"server"`) || strings.Contains(body, `"database"`))
}

func TestGetConfigDoesNotExposeServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/config", NewConfigHandler(&fakeConfigHandlerService{
		err: errors.New("connect postgres://user:database-secret@example.test failed"),
	}).GetConfig)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config", nil))

	require.Equal(t, http.StatusInternalServerError, response.Code)
	assert.NotContains(t, response.Body.String(), "database-secret")
}

type fakeConfigHandlerService struct {
	service.ConfigService
	cfg *config.Config
	err error
}

func (f *fakeConfigHandlerService) GetConfig() (*config.Config, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.cfg, nil
}
