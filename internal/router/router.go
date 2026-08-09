package router

import (
	"github.com/gin-gonic/gin"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	"go.uber.org/zap"
)

func Init(zapLogger *zap.SugaredLogger) *gin.Engine {

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middlewares.RequestLogger(zapLogger))
	versioned := r.Group("/api/v1")
	versioned.GET("hello")

	return r
}
