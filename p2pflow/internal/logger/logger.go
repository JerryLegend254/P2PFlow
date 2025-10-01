package logger

import "go.uber.org/zap"

type Logger struct {
	*zap.SugaredLogger
}

type LoggerType string

const (
	JSON    LoggerType = "json"
	CONSOLE LoggerType = "json"
)

func NewLogger(t LoggerType) Logger {
	switch t {
	case CONSOLE:
		consoleCfg := zap.NewDevelopmentConfig()
		consoleCfg.EncoderConfig.TimeKey = ""
		consoleCfg.EncoderConfig.LevelKey = ""
		consoleCfg.EncoderConfig.CallerKey = ""
		consoleCfg.Encoding = "console"
		consoleLogger, _ := consoleCfg.Build()
		return Logger{consoleLogger.Sugar()}
	}
	return Logger{zap.Must(zap.NewProduction()).Sugar()}
}
