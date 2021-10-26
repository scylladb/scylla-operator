// Copyright (C) 2017 ScyllaDB

package log

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Mode specifies logs destination.
type Mode int8

const (
	// StderrMode logs are written to standard error.
	StderrMode Mode = iota
	// StderrMode logs are written to standard output.
	StdoutMode
)

func (m Mode) String() string {
	switch m {
	case StderrMode:
		return "stderr"
	case StdoutMode:
		return "stdout"
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
	case "stdout":
		*m = StdoutMode
	default:
		return fmt.Errorf("unrecognized mode: %q", string(text))
	}

	return nil
}

// Config specifies log mode and level.
type Config struct {
	Mode     Mode                `json:"mode" yaml:"mode"`
	Level    zap.AtomicLevel     `json:"level" yaml:"level"`
	Sampling *zap.SamplingConfig `json:"sampling" yaml:"sampling"`
}

// NewProduction builds a production Logger based on the configuration.
func NewProduction(c Config, opts ...zap.Option) (Logger, error) {
	enc := zapcore.EncoderConfig{
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

	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig = enc
	cfg.OutputPaths = []string{c.Mode.String()}
	cfg.Sampling = c.Sampling
	cfg.Level = c.Level
	cfg.DisableCaller = true

	l, err := cfg.Build(opts...)
	if err != nil {
		return NopLogger, err
	}
	return NewLogger(l), nil
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
