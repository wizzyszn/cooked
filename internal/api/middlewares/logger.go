package middlewares

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RequestLogger(zapLogger *zap.SugaredLogger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		path := ctx.Request.URL.Path
		raw := ctx.Request.URL.RawQuery
		ctx.Next()

		if raw != "" {
			path += "?" + raw
		}

		reqID, _ := ctx.Get(RequestIdKey)

		zapLogger.Infow("http_request", "status", ctx.Writer.Status(), "method", ctx.Request.Method, "path", path, "latency_ms", time.Since(start).Milliseconds(), "client_ip", ctx.ClientIP(), "requestId", reqID, "erros", ctx.Errors.ByType(gin.ErrorTypePrivate))

	}
}
