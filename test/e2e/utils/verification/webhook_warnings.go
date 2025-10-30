package verification

import (
	"context"

	"k8s.io/client-go/rest"
)

// warningCaptureHandlerContextKeyType is the context key used to store the pointer to the warning string.
type warningCaptureHandlerContextKeyType struct{}

// WarningCaptureHandler allows capturing warnings returned by the API server during tests
// by storing them in a string pointer stored in the context.
type WarningCaptureHandler struct {
}

func RestConfigWithWarningCaptureHandler(cfg *rest.Config) *rest.Config {
	cfg.WarningHandlerWithContext = &WarningCaptureHandler{}
	return cfg
}

// HandleWarningHeaderWithContext implements rest.WarningHandlerWithContext.
// It retrieves a pointer to a string from the context and updates it with the warning message.
// This allows for concurrent execution of tests with a shared client (enforced by the unfortunate framework implementation).
func (h *WarningCaptureHandler) HandleWarningHeaderWithContext(ctx context.Context, _ int, _ string, text string) {
	v := ctx.Value(warningCaptureHandlerContextKeyType{})
	if warningPtr, ok := v.(*string); ok {
		*warningPtr = text
	}
}

// WarningContext is a context that can capture warnings from the API server
// (when used with a rest.Config that has WarningCaptureHandler as its WarningHandler).
type WarningContext struct {
	context.Context
	warning string
}

// NewWarningContext creates a new WarningContext based on the provided context.
func NewWarningContext(ctx context.Context) *WarningContext {
	wCtx := &WarningContext{}
	wCtx.Context = context.WithValue(ctx, warningCaptureHandlerContextKeyType{}, &wCtx.warning)
	return wCtx
}

// CapturedWarning returns the warning message captured from the API server, if any.
func (w *WarningContext) CapturedWarning() string {
	return w.warning
}
