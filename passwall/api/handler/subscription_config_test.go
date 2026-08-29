package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"passwall/internal/model"
	"passwall/internal/service/proxy"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveSubscriptionConfigReturnsServiceStatus(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "scheduler failure", err: errors.New("scheduler unavailable"), want: http.StatusInternalServerError},
		{name: "missing subscription", err: proxy.ErrSubscriptionNotFound, want: http.StatusNotFound},
		{name: "invalid cron", err: proxy.ErrInvalidSubscriptionConfig, want: http.StatusUnprocessableEntity},
	} {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			manager := &fakeSubscriptionConfigManager{saveErr: test.err}
			router := gin.New()
			router.POST("/subscription/:id/config", SaveSubscriptionConfig(manager))
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/subscription/7/config", strings.NewReader(`{
				"auto_update": true,
				"update_interval": "0 0 0 * * *",
				"use_proxy": false
			}`))
			request.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(response, request)

			require.Equal(t, test.want, response.Code)
			assert.NotNil(t, manager.saved)
		})
	}
}

type fakeSubscriptionConfigManager struct {
	proxy.SubscriptionManager
	saved   *model.SubscriptionConfig
	saveErr error
}

func (f *fakeSubscriptionConfigManager) SaveSubscriptionConfig(config *model.SubscriptionConfig) error {
	f.saved = config
	return f.saveErr
}
