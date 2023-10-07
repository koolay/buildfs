package logging

import (
	"io"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

type FileLogConfig struct {
	LogFilePath   string
	LogMaxSize    int
	LogMaxBackups int
	LogMaxAge     int
}

func newFileWriter(cfg *FileLogConfig) io.Writer {
	return &lumberjack.Logger{
		Filename:   cfg.LogFilePath,
		MaxSize:    cfg.LogMaxSize,
		MaxBackups: cfg.LogMaxBackups,
		MaxAge:     cfg.LogMaxAge,
	}
}
