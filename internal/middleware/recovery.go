package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/Tencent/XinWiki/internal/logger"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				ctx := c.Request.Context()
				requestID, _ := c.Get("RequestID")

				stacktrace := debug.Stack()
				logger.ErrorWithFields(ctx, fmt.Errorf("panic: %v", err), logrus.Fields{
					"request_id": requestID,
					"stacktrace": string(stacktrace),
				})

				resp := gin.H{"error": "Internal Server Error"}
				if gin.Mode() != gin.ReleaseMode {
					resp["message"] = fmt.Sprintf("%v", err)
				}
				c.AbortWithStatusJSON(500, resp)
			}
		}()

		c.Next()
	}
}
