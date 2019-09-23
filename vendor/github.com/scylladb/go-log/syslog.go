// Copyright (C) 2017 ScyllaDB

package log

import (
	"log/syslog"

	"go.uber.org/zap/zapcore"
)

// SyslogCore is a zapcore.Core that logs to syslog.
type SyslogCore struct {
	zapcore.LevelEnabler
	enc zapcore.Encoder
	out *syslog.Writer
}

// NewSyslogCore creates a Core that writes logs to a syslog.
func NewSyslogCore(enc zapcore.Encoder, out *syslog.Writer, enab zapcore.LevelEnabler) *SyslogCore {
	return &SyslogCore{
		LevelEnabler: enab,
		enc:          enc,
		out:          out,
	}
}

// With implements zapcore.Core.
func (c *SyslogCore) With(fields []zapcore.Field) zapcore.Core {
	clone := c.clone()
	addFields(clone.enc, fields)
	return clone
}

// Check implements zapcore.Core.
func (c *SyslogCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

// Write implements zapcore.Core.
func (c *SyslogCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	buf, err := c.enc.EncodeEntry(ent, fields)
	if err != nil {
		return err
	}

	switch ent.Level {
	case zapcore.DebugLevel:
		err = c.out.Debug(buf.String())
	case zapcore.InfoLevel:
		err = c.out.Info(buf.String())
	case zapcore.WarnLevel:
		err = c.out.Warning(buf.String())
	case zapcore.ErrorLevel:
		err = c.out.Err(buf.String())
	case zapcore.DPanicLevel:
		err = c.out.Crit(buf.String())
	case zapcore.PanicLevel:
		err = c.out.Crit(buf.String())
	case zapcore.FatalLevel:
		err = c.out.Crit(buf.String())
	default:
		_, err = c.out.Write(buf.Bytes())
	}
	buf.Free()
	if err != nil {
		return err
	}
	if ent.Level > zapcore.ErrorLevel {
		// Since we may be crashing the program, sync the output. Ignore Sync
		// errors, pending a clean solution to issue #370.
		c.Sync()
	}
	return nil
}

// Sync implements zapcore.Core. It closes the underlying connection to syslog
// daemon.
func (c *SyslogCore) Sync() error {
	return c.out.Close()
}

func (c *SyslogCore) clone() *SyslogCore {
	return &SyslogCore{
		LevelEnabler: c.LevelEnabler,
		enc:          c.enc.Clone(),
		out:          c.out,
	}
}

func addFields(enc zapcore.ObjectEncoder, fields []zapcore.Field) {
	for i := range fields {
		fields[i].AddTo(enc)
	}
}
