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

func TestUpdateConfigRejectsUnknownAndWronglyTypedFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "unknown", body: `{"concurent":5}`},
		{name: "wrong type", body: `{"concurrent":"five"}`},
		{name: "unknown nested", body: `{"proxy":{"enabled":true,"typo":1}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeConfigHandlerService{}
			router := gin.New()
			router.POST("/config", NewConfigHandler(service).UpdateConfig)
			request := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusBadRequest, response.Code)
			assert.Zero(t, service.updateCalls)
		})
	}
}

func TestUpdateConfigRejectsEmptyPatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeConfigHandlerService{}
	router := gin.New()
	router.POST("/config", NewConfigHandler(service).UpdateConfig)
	request := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
	assert.Zero(t, service.updateCalls)
}

func TestUpdateConfigReturnsSemanticAndInternalStatuses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "semantic", err: service.ErrInvalidConfig, want: http.StatusUnprocessableEntity},
		{name: "internal", err: errors.New("database failed"), want: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeConfigHandlerService{updateErr: test.err}
			router := gin.New()
			router.POST("/config", NewConfigHandler(service).UpdateConfig)
			request := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(`{"concurrent":5}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, test.want, response.Code)
			assert.Equal(t, 1, service.updateCalls)
		})
	}
}

func TestUpdateConfigAcceptsTypedFrontendPatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeConfigHandlerService{}
	router := gin.New()
	router.POST("/config", NewConfigHandler(service).UpdateConfig)
	body := `{
		"clash_api":{"enable":true,"clients":[{"existing_index":0,"url":"","secret":""}]},
		"cron_jobs":[{"existing_index":0,"name":"job","schedule":"0 0 4 * * *","test_proxy":{},"auto_ban":{},"ip_check":{},"webhook":[{"existing_index":0,"name":"hook","method":"POST","url":"","header":"","body":""}]}]
	}`
	request := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, 1, service.updateCalls)
	assert.Contains(t, service.updates, "clash_api")
	assert.Contains(t, service.updates, "cron_jobs")
}

type fakeConfigHandlerService struct {
	service.ConfigService
	cfg         *config.Config
	err         error
	updateErr   error
	updates     map[string]interface{}
	updateCalls int
}

func (f *fakeConfigHandlerService) GetConfig() (*config.Config, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.cfg, nil
}

func (f *fakeConfigHandlerService) UpdateConfig(updates map[string]interface{}) error {
	f.updateCalls++
	f.updates = updates
	return f.updateErr
}
