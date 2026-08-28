package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"passwall/internal/model"
	"passwall/internal/service/proxy"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSubscriptionsListShowsSafeSourceWithoutSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	subscription := &model.Subscription{
		ID:      7,
		URL:     "https://user:password@example.com/sub?token=url-secret",
		Content: "raw-subscription-secret",
		Status:  model.SubscriptionStatusOK,
	}
	router := gin.New()
	router.GET("/subscriptions", GetSubscriptions(
		&fakeSafeSubscriptionManager{subscription: subscription},
		&fakeSafeSubscriptionProxyService{},
	))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/subscriptions?page=1&pageSize=10", nil))

	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.NotContains(t, body, "url-secret")
	assert.NotContains(t, body, "password")
	assert.NotContains(t, body, "raw-subscription-secret")
	assert.NotContains(t, body, `"url"`)
	assert.NotContains(t, body, `"content"`)
	assert.Contains(t, body, `"refreshable":true`)
	assert.Contains(t, body, `"source":"example.com"`)
}

func TestGetSubscriptionDetailShowsURLWithoutRawContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	subscription := &model.Subscription{
		ID:      7,
		URL:     "https://user:password@example.com/sub?token=url-secret",
		Content: "raw-subscription-secret",
		Status:  model.SubscriptionStatusOK,
	}
	router := gin.New()
	router.GET("/subscriptions", GetSubscriptions(
		&fakeSafeSubscriptionManager{subscription: subscription},
		&fakeSafeSubscriptionProxyService{},
	))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/subscriptions?id=7", nil))

	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `"url":"https://user:password@example.com/sub?token=url-secret"`)
	assert.NotContains(t, body, "raw-subscription-secret")
	assert.NotContains(t, body, `"content"`)
}

type fakeSafeSubscriptionManager struct {
	proxy.SubscriptionManager
	subscription *model.Subscription
}

func (f *fakeSafeSubscriptionManager) GetSubscriptionByID(uint) (*model.Subscription, error) {
	return f.subscription, nil
}

func (f *fakeSafeSubscriptionManager) GetSubscriptionsPage(proxy.SubsPage) ([]*model.Subscription, int64, error) {
	return []*model.Subscription{f.subscription}, 1, nil
}

type fakeSafeSubscriptionProxyService struct {
	proxy.ProxyService
}

func (*fakeSafeSubscriptionProxyService) GetProxyNumBySubscriptionID(uint, bool, bool) (int64, error) {
	return 1, nil
}
