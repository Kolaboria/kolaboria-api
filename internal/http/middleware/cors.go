package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORS(allowOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			allowed := false

			if len(allowOrigins) == 0 {
				// Allow all origins when the list is empty (development)
				c.Header("Access-Control-Allow-Origin", "*")
				allowed = true
			} else {
				// Check if the request's origin is in the allowed list
				for _, o := range allowOrigins {
					if o == origin {
						c.Header("Access-Control-Allow-Origin", origin)
						c.Header("Vary", "Origin")
						allowed = true
						break
					}
				}
			}

			if allowed {
				c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
				c.Header("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Request-ID, X-Trace-ID")
				c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, X-Request-ID, X-Trace-ID")
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
