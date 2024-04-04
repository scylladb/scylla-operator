package log

import (
	"fmt"
	"io"

	pkgErrors "github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func stringifyErrors(fields []zapcore.Field, withStack bool) []zapcore.Field {
	var stacks []zapcore.Field

	for i, f := range fields {
		if f.Type == zapcore.ErrorType {
			if withStack {
				if s, ok := fields[i].Interface.(stackTracer); ok {
					stacks = append(stacks, zap.String(f.Key+"Stack", fmt.Sprintf("%+v", stackPrinter{s.StackTrace()})))
				}
			}

			fields[i].Type = zapcore.StringType
			fields[i].String = f.Interface.(error).Error()
			fields[i].Interface = nil
		}
	}

	return append(fields, stacks...)
}

type stackTracer interface {
	StackTrace() pkgErrors.StackTrace
}

type stackPrinter struct {
	stack []pkgErrors.Frame
}

func (p stackPrinter) Format(s fmt.State, verb rune) {
	for _, f := range p.stack {
		f.Format(s, 'v')
		io.WriteString(s, "\n")
	}
}
