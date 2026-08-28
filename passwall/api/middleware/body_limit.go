package middleware

import (
	"bytes"
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
)

const maxJSONBodySize = 1 << 20

func RequestBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		mediaType, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if c.Request.Body == nil || c.Request.Body == http.NoBody || mediaType == "multipart/form-data" {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxJSONBodySize {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxJSONBodySize+1))
		_ = c.Request.Body.Close()
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if len(body) > maxJSONBodySize {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}
