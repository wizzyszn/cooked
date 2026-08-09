package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIdKey = "requestId"
const RequestHeaderKey = "X-Request-ID"

func RequestId() gin.HandlerFunc {
	return func(c *gin.Context) {

		reqID := c.GetHeader(RequestHeaderKey)
		if reqID == "" {
			reqID = uuid.NewString()
		}
		c.Set(RequestIdKey, reqID)
		c.Header(RequestHeaderKey, reqID)

		c.Next()
	}
}
