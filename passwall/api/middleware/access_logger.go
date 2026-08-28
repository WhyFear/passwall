package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessLogger records only fields that cannot contain request credentials.
func AccessLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "<unmatched>"
		}
		fmt.Fprintf(gin.DefaultWriter, "[GIN] method=%s status=%d latency=%s route=%s\n",
			c.Request.Method, c.Writer.Status(), time.Since(started), route)
	}
}
