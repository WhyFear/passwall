package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAccessLoggerUsesRouteTemplateAndOmitsCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	oldWriter := gin.DefaultWriter
	gin.DefaultWriter = &output
	defer func() { gin.DefaultWriter = oldWriter }()

	router := gin.New()
	router.Use(AccessLogger())
	router.GET("/s/:slug", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/s/real-share-secret?token=query-secret", nil)
	request.Header.Set("Authorization", "Bearer header-secret")
	router.ServeHTTP(httptest.NewRecorder(), request)

	logs := output.String()
	assert.Contains(t, logs, "method=GET")
	assert.Contains(t, logs, "status=204")
	assert.Contains(t, logs, "route=/s/:slug")
	for _, secret := range []string{"real-share-secret", "query-secret", "header-secret", "Authorization"} {
		assert.NotContains(t, logs, secret)
	}
}
