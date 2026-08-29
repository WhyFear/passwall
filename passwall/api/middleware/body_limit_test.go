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
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		called = true
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

func TestRequestBodyLimitDoesNotTrustMultipartContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestBodyLimit())
	router.POST("/config", func(c *gin.Context) { _, _ = io.Copy(io.Discard, c.Request.Body) })

	request := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader(make([]byte, maxJSONBodySize+1)))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=fake")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
}
