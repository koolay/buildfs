package logging

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
)

const defaultLogLevel = "info"

type Config struct {
	Level         string
	fileLogConfig *FileLogConfig
}

func NewTestLog() logr.Logger {
	zlvl, _ := zerolog.ParseLevel("info")
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}
	consoleWriter.NoColor = true
	zlogger := zerolog.New(consoleWriter).Level(zlvl).With().Timestamp().Logger()

	return zerologr.New(&zlogger)
}

func NewLogger(cfg Config) (logr.Logger, error) {
	// disable "v=<log-level>" field on every log line
	zerologr.VerbosityFieldName = ""
	// don't display floats for time durations
	zerolog.DurationFieldInteger = true
	level := cfg.Level
	if level == "" {
		level = defaultLogLevel
	}

	zlvl, err := zerolog.ParseLevel(level)
	if err != nil {
		return logr.Logger{}, err
	}

	var output io.Writer

	if cfg.fileLogConfig != nil {
		rollingLogger := newFileWriter(cfg.fileLogConfig)
		output = io.MultiWriter(os.Stdout, rollingLogger)
	} else {
		output = os.Stdout
	}

	consoleWriter := zerolog.ConsoleWriter{
		Out:        output,
		TimeFormat: time.RFC3339,
	}
	// insert a delimiter, '|', between the message and the logfmt key values,
	// to aid log parsing
	consoleWriter.FormatMessage = func(msg interface{}) string {
		return fmt.Sprintf(`%s |`, msg)
	}

	if isatty.IsTerminal(os.Stdout.Fd()) {
		consoleWriter.NoColor = false
	}

	zlogger := zerolog.New(consoleWriter).Level(zlvl).With().Timestamp().Logger()

	// wrap within logr wrapper
	return zerologr.New(&zlogger), nil
}
