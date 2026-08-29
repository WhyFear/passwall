package middleware

import (
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
)

const maxJSONBodySize = 1 << 20
const maxCreateProxyBodySize = 12 << 20

func RequestBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil || c.Request.Body == http.NoBody {
			c.Next()
			return
		}
		limit := int64(maxJSONBodySize)
		mediaType, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if mediaType == "multipart/form-data" && (c.FullPath() == "/api/v1/create_proxy" || c.FullPath() == "/web/api/create_proxy") {
			limit = maxCreateProxyBodySize
		}
		if c.Request.ContentLength > limit {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
