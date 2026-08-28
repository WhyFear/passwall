package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"passwall/config"
	"passwall/internal/adapter/parser"
	"passwall/internal/model"
	"passwall/internal/service"
	"passwall/internal/service/proxy"

	"github.com/gin-gonic/gin"
	"github.com/metacubex/mihomo/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubProcessorRejectsProxyPersistenceFailure(t *testing.T) {
	subscriptions := &fakeCreateSubscriptionManager{}
	processor := &subProcessor{
		proxyService:        &fakeCreateProxyService{err: errors.New("insert failed")},
		subscriptionManager: subscriptions,
		parserFactory: &fakeCreateParserFactory{parser: &fakeCreateParser{
			proxies: []*model.Proxy{{Name: "node", Domain: "127.0.0.1", Port: 443}},
		}},
	}

	sub, count, err := processor.run("test://subscription", "fake", []byte("content"))

	require.ErrorContains(t, err, "insert failed")
	require.NotNil(t, sub)
	assert.Zero(t, count)
	assert.Equal(t, model.SubscriptionStatusInvalid, subscriptions.status)
}

func TestCreateProxyReportsFailureAfterSubscriptionWasCreated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	subscriptions := &fakeCreateSubscriptionManager{}
	handler := CreateProxy(
		&fakeCreateProxyService{},
		subscriptions,
		&fakeCreateParserFactory{parser: &fakeCreateParser{err: errors.New("bad subscription")}},
		fakeCreateProxyTester{},
		fakeCreateIPDetector{},
		fakeCreateConfigService{},
	)
	router := gin.New()
	router.POST("/create_proxy", handler)
	body := bytes.NewBufferString(`{"url":"test://subscription","type":"fake"}`)
	request := httptest.NewRequest(http.MethodPost, "/create_proxy", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	assert.Equal(t, "fail", result["result"])
	assert.Equal(t, float64(http.StatusBadRequest), result["status_code"])
	assert.Equal(t, model.SubscriptionStatusInvalid, subscriptions.status)
}

func TestCreateProxyDownloadFailureDoesNotLogCredentialURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	events := log.Subscribe()
	defer log.UnSubscribe(events)
	handler := CreateProxy(
		&fakeCreateProxyService{},
		&fakeCreateSubscriptionManager{},
		&fakeCreateParserFactory{},
		fakeCreateProxyTester{},
		fakeCreateIPDetector{},
		fakeCreateConfigService{},
	)
	router := gin.New()
	router.POST("/create_proxy", handler)
	body := bytes.NewBufferString(`{"url":"https://user:password@%41?token=create-url-secret","type":"fake"}`)
	request := httptest.NewRequest(http.MethodPost, "/create_proxy", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	assert.NotContains(t, response.Body.String(), "create-url-secret")
	select {
	case event := <-events:
		assert.NotContains(t, strings.ToLower(event.Payload), "create-url-secret")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for import log")
	}
}

type fakeCreateParserFactory struct {
	parser.ParserFactory
	parser parser.Parser
}

func (f *fakeCreateParserFactory) GetParser(string, []byte) (parser.Parser, error) {
	return f.parser, nil
}

type fakeCreateParser struct {
	proxies []*model.Proxy
	err     error
}

func (f *fakeCreateParser) Parse([]byte) ([]*model.Proxy, error) { return f.proxies, f.err }
func (f *fakeCreateParser) CanParse([]byte) bool                 { return true }
func (f *fakeCreateParser) GetType() model.SubscriptionType      { return model.SubscriptionTypeClash }

type fakeCreateSubscriptionManager struct {
	proxy.SubscriptionManager
	status model.SubscriptionStatus
}

func (f *fakeCreateSubscriptionManager) GetSubscriptionByURL(string) (*model.Subscription, error) {
	return nil, nil
}

func (f *fakeCreateSubscriptionManager) CreateSubscription(subscription *model.Subscription) error {
	subscription.ID = 42
	return nil
}

func (f *fakeCreateSubscriptionManager) UpdateSubscriptionStatus(subscription *model.Subscription) error {
	f.status = subscription.Status
	return nil
}

type fakeCreateProxyService struct {
	proxy.ProxyService
	err error
}

func (f *fakeCreateProxyService) BatchCreateProxies([]*model.Proxy) error { return f.err }

type fakeCreateProxyTester struct{}

func (fakeCreateProxyTester) TestProxies(*service.TestProxyRequest, bool) error { return nil }

type fakeCreateIPDetector struct {
	service.IPDetectorService
}

func (fakeCreateIPDetector) BatchDetect(context.Context, *service.BatchIPDetectorReq) error {
	return nil
}

type fakeCreateConfigService struct {
	service.ConfigService
}

func (fakeCreateConfigService) GetConfig() (*config.Config, error) {
	return &config.Config{Concurrent: 1}, nil
}
