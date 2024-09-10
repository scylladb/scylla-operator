package systemd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/util/rand"
)

func hasSystemdRunning() (bool, error) {
	const systemdRuntimeDir = "/run/systemd/system"
	_, err := os.Stat(systemdRuntimeDir)
	switch {
	case os.IsNotExist(err):
		return false, nil
	case err == nil:
		return true, nil
	default:
		return false, fmt.Errorf("can't stat path %q to detect whether systemd is running: %w", systemdRuntimeDir, err)
	}
}

func TestSystemdControl_ErrNotExist(t *testing.T) {
	// FIXME: We should either use fake systemd, or somehow enable it.
	//        Ref: https://github.com/scylladb/scylla-operator/issues/1379
	systemdRunning, err := hasSystemdRunning()
	if err != nil {
		t.Fatal(err)
	}
	if !systemdRunning {
		t.Skip("systemd is not available, skipping the test")
	}

	verifyError := func(err error) {
		t.Helper()

		if !errors.Is(err, ErrNotExist) {
			t.Errorf("expected error to contain %q, got %#v", ErrNotExist.Error(), err)
		}
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	sc, err := NewSystemdUserControl(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sc.Close()

	notExistingUnitName := fmt.Sprintf("%s.mount", rand.String(32))

	err = sc.EnableUnits(ctx, []string{notExistingUnitName})
	verifyError(err)

	err = sc.DisableUnits(ctx, []string{notExistingUnitName})
	verifyError(err)

	err = sc.StartUnit(ctx, notExistingUnitName)
	verifyError(err)

	err = sc.StopUnit(ctx, notExistingUnitName)
	verifyError(err)
}
