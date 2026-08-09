package logger

import "go.uber.org/zap"

var log *zap.SugaredLogger

func Init(env string) *zap.SugaredLogger {
	var err error
	var logger *zap.Logger
	if env == "production" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		panic("failed to initialize logger:" + err.Error())
	}

	log = logger.Sugar()

	return log
}

func Get() *zap.SugaredLogger {
	if log == nil {
		Init("development")
	}
	return log
}
