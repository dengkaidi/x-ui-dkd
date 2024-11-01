package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	log "github.com/sirupsen/logrus"
)

var logger *log.Logger

func init() {
	InitLogger(log.InfoLevel)
}

func InitLogger(level log.Level) {
	logger = log.New()
	logger.SetOutput(os.Stderr)

	logger.SetFormatter(&log.TextFormatter{
		DisableColors:   false,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		// 启用短文件名和行号
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			return filepath.Base(f.File), fmt.Sprintf("%d", f.Line)
		},
	})

	logger.ReportCaller = true
	logger.Level = level
}

func Debug(args ...interface{}) {
	logger.Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

func Info(args ...interface{}) {
	logger.Info(args...)
}

func Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

func Warning(args ...interface{}) {
	logger.Warning(args...)
}

func Warningf(format string, args ...interface{}) {
	logger.Warningf(format, args...)
}

func Error(args ...interface{}) {
	logger.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}
