package verification

import (
	"context"
	"testing"
)

func TestWarningCaptureHandler(t *testing.T) {
	h := &WarningCaptureHandler{}

	warningCtx := NewWarningContext(context.Background())
	if warningCtx.CapturedWarning() != "" {
		t.Errorf("Expected empty warning, got: %s", warningCtx.CapturedWarning())
	}

	const testWarning = "This is a test warning"
	h.HandleWarningHeaderWithContext(warningCtx, 299, "299", testWarning)

	if warningCtx.CapturedWarning() != testWarning {
		t.Errorf("Expected warning: %s, got: %s", testWarning, warningCtx.CapturedWarning())
	}
}
