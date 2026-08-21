package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"passwall/internal/model"
	"passwall/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectMissingIPQualityStartsTaskWithUniqueTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	detector := &fakeMissingIPDetector{total: 3}
	router := gin.New()
	router.POST("/detect_missing_ip", DetectMissingIPQuality(context.Background(), detector))
	body := bytes.NewBufferString(`{"type":["ss","","ss","trojan"]}`)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/detect_missing_ip", body))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []model.ProxyType{model.ProxyTypeSS, model.ProxyTypeTrojan}, detector.types)
	assert.True(t, detector.async)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "success", response["result"])
	assert.Equal(t, float64(3), response["total"])
}

type fakeMissingIPDetector struct {
	service.IPDetectorService
	types []model.ProxyType
	async bool
	total int
	err   error
}

func (f *fakeMissingIPDetector) DetectMissing(_ context.Context, types []model.ProxyType, async bool) (int, error) {
	f.types = types
	f.async = async
	return f.total, f.err
}
