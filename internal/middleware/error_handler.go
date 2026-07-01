package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/XinWiki/internal/errors"
	"github.com/Tencent/XinWiki/internal/logger"
)

// ErrorHandler is a middleware that converts AppErrors attached to c.Errors
// into a consistent JSON error response. In release mode it also scrubs
// overly verbose internal-error details to prevent leaking SQL fragments,
// file paths, or stack traces to clients — full details are still written
// to the server-side log.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}
		// Log all collected errors server-side before producing a response,
		// so even sanitized client messages retain their diagnostic detail
		// in the logs.
		for _, e := range c.Errors {
			if e != nil && e.Err != nil {
				logger.ErrorWithFields(c.Request.Context(), e.Err, logger.Fields{
					"path":   c.Request.URL.Path,
					"method": c.Request.Method,
					"status": c.Writer.Status(),
				})
			}
		}

		err := c.Errors.Last().Err

		if appErr, ok := errors.IsAppError(err); ok {
			c.JSON(appErr.HTTPCode, gin.H{
				"success": false,
				"error": gin.H{
					"code":    appErr.Code,
					"message": appErr.Message,
					"details": appErr.Details,
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    errors.ErrInternalServer,
				"message": "Internal server error",
			},
		})
	}
}
