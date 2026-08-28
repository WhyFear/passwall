package handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"passwall/internal/service/proxy"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPinProxyStopsAfterServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/pin", PinProxy(&fakePinProxyService{err: errors.New("update failed")}))
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/pin", bytes.NewBufferString(`{"id":1,"pinned":true}`))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code)
	assert.JSONEq(t, `{"result":"fail","status_code":500,"status_msg":"Failed to pin proxy"}`, response.Body.String())
}

type fakePinProxyService struct {
	proxy.ProxyService
	err error
}

func (f *fakePinProxyService) PinProxy(uint, bool) error { return f.err }
