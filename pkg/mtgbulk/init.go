package mtgbulk

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
