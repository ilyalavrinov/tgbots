package mtgbulkbuy

import "go.uber.org/zap"

var loggerRaw *zap.Logger
var logger *zap.SugaredLogger

func init() {
	var err error
	cfg := zap.NewDevelopmentConfig()
	cfg.Development = false
	loggerRaw, err = cfg.Build()
	if err != nil {
		panic(err)
	}
	logger = loggerRaw.Sugar()
}

func Debugw(msg string, keysAndValues ...interface{}) {
	logger.Debugw(msg, keysAndValues...)
}

func Fatalw(msg string, keysAndValues ...interface{}) {
	logger.Fatalw(msg, keysAndValues...)
}

func Info(msg string) {
	logger.Info(msg)
}

func Errorw(msg string, keysAndValues ...interface{}) {
	logger.Errorw(msg, keysAndValues...)
}
