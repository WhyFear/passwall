package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestBodyLimitRejectsChunkedBodyOverOneMiBAndPreservesValidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	router := gin.New()
	router.Use(RequestBodyLimit())
	router.POST("/", func(c *gin.Context) {
		called = true
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		c.Data(http.StatusOK, "application/json", body)
	})

	valid := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"ok":true}`))
	valid.Header.Set("Content-Type", "application/json")
	validResponse := httptest.NewRecorder()
	router.ServeHTTP(validResponse, valid)
	require.Equal(t, http.StatusOK, validResponse.Code)
	assert.Equal(t, `{"ok":true}`, validResponse.Body.String())

	called = false
	tooLarge := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(make([]byte, maxJSONBodySize+1)))
	tooLarge.Header.Set("Content-Type", "application/json")
	tooLarge.ContentLength = -1
	tooLargeResponse := httptest.NewRecorder()
	router.ServeHTTP(tooLargeResponse, tooLarge)
	assert.Equal(t, http.StatusRequestEntityTooLarge, tooLargeResponse.Code)
	assert.False(t, called)
}
