package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/proxy"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetTypesReturnsProxyTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/types", GetTypes(&fakeProxyService{types: []string{"vmess", "trojan"}}))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/types", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.JSONEq(t, `["vmess","trojan"]`, resp.Body.String())
}

func TestGetTypesReturnsServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/types", GetTypes(&fakeProxyService{err: errors.New("db failed")}))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/types", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, resp.Body.String(), "Failed to get proxy types")
}

type fakeProxyService struct {
	types []string
	err   error
}

func (f *fakeProxyService) GetProxyByID(id uint) (*model.Proxy, error) {
	return nil, nil
}

func (f *fakeProxyService) GetProxyNumBySubscriptionID(subsId uint, ignoreBanned bool, statusOK bool) (int64, error) {
	return 0, nil
}

func (f *fakeProxyService) GetProxiesByFilters(filters *repository.NodeFilter, sort string, sortOrder string, page int, pageSize int) ([]*model.Proxy, int64, error) {
	return nil, 0, nil
}

func (f *fakeProxyService) GetProxyByName(name string) (*model.Proxy, error) {
	return nil, nil
}

func (f *fakeProxyService) CreateProxy(proxy *model.Proxy) error {
	return nil
}

func (f *fakeProxyService) BatchCreateProxies(proxies []*model.Proxy) error {
	return nil
}

func (f *fakeProxyService) GetTypes() ([]string, error) {
	return f.types, f.err
}

func (f *fakeProxyService) PinProxy(id uint, pin bool) error {
	return nil
}

func (f *fakeProxyService) BanProxy(ctx context.Context, req proxy.BanProxyReq) error {
	return nil
}
