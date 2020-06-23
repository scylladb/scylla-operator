// Copyright (C) 2017 ScyllaDB

package log

import (
	"errors"
	"fmt"
	"log/syslog"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Mode specifies logs destination.
type Mode int8

const (
	// StderrMode logs are written to standard error.
	StderrMode Mode = iota
	// SyslogMode logs are written to syslog with priority LOG_DAEMON and
	// tag os.Args[0].
	SyslogMode
)

func (m Mode) String() string {
	switch m {
	case StderrMode:
		return "stderr"
	case SyslogMode:
		return "syslog"
	}

	return ""
}

// MarshalText implements encoding.TextMarshaler.
func (m Mode) MarshalText() ([]byte, error) {
	return []byte(m.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (m *Mode) UnmarshalText(text []byte) error {
	switch string(text) {
	case "stderr", "STDERR":
		*m = StderrMode
	case "syslog", "SYSLOG":
		*m = SyslogMode
	default:
		return fmt.Errorf("unrecognized mode: %q", string(text))
	}

	return nil
}

// Config specifies log mode and level.
type Config struct {
	Mode  Mode                 `yaml:"mode"`
	Level zapcore.LevelEnabler `yaml:"level"`
}

// NewProduction builds a production Logger based on the configuration.
func NewProduction(c Config, opts ...zap.Option) (Logger, error) {
	opts = append([]zap.Option{zap.ErrorOutput(os.Stderr)}, opts...)

	cfg := zapcore.EncoderConfig{
		// Keys can be anything except the empty string.
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var core zapcore.Core
	switch c.Mode {
	case StderrMode:
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(cfg),
			zapcore.Lock(os.Stderr),
			c.Level,
		)
	case SyslogMode:
		w, err := syslog.New(syslog.LOG_DAEMON, "")
		if err != nil {
			return NopLogger, err
		}

		// ignore level and time as they will be logged by syslog
		cfg.LevelKey = ""
		cfg.TimeKey = ""

		core = NewSyslogCore(
			zapcore.NewJSONEncoder(cfg),
			w,
			c.Level,
		)
	default:
		return NopLogger, errors.New("unrecognized logger mode")
	}

	return Logger{base: zap.New(core, opts...)}, nil
}

// NewDevelopment creates a new logger that writes DebugLevel and above
// logs to standard error in a human-friendly format.
func NewDevelopment() Logger {
	return NewDevelopmentWithLevel(zapcore.DebugLevel)
}

// NewDevelopmentWithLevel creates a new logger that writes level and above
// logs to standard error in a human-friendly format.
func NewDevelopmentWithLevel(level zapcore.Level) Logger {
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncoderConfig.EncodeTime = shortTimeEncoder
	cfg.EncoderConfig.CallerKey = ""
	cfg.Level.SetLevel(level)
	l, _ := cfg.Build()
	return Logger{base: l}
}

func shortTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("15:04:05.000"))
}
