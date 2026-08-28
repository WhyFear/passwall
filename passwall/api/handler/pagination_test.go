package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestPaginatedHandlersRejectInvalidPageSizes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		path    string
		handler gin.HandlerFunc
	}{
		{"proxies over max", "/proxies?pageSize=201", GetProxyList(&fakeListProxyService{})},
		{"proxies negative", "/proxies?pageSize=-1", GetProxyList(&fakeListProxyService{})},
		{"subscriptions over max", "/subscriptions?pageSize=201", GetSubscriptions(&fakeSafeSubscriptionManager{subscription: &model.Subscription{}}, &fakeSafeSubscriptionProxyService{})},
		{"history over max", "/proxy/1/history?pageSize=201", GetProxyHistory(&fakeHistoryService{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			route, _, _ := strings.Cut(tt.path, "?")
			router.GET(route, tt.handler)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tt.path, nil))
			assert.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}

type fakeHistoryService struct {
	service.SpeedTestHistoryService
}

func (*fakeHistoryService) GetSpeedTestHistoryByProxyID(uint, *repository.PageQuery) (repository.SpeedTestHistoryPageResult, error) {
	return repository.SpeedTestHistoryPageResult{}, nil
}
