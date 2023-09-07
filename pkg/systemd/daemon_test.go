package systemd

import (
	"context"
	"errors"
	"fmt"
	"testing"

	godbus "github.com/godbus/dbus/v5"
	"k8s.io/apimachinery/pkg/util/rand"
)

func TestSystemdControl_ErrNotExist(t *testing.T) {
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

	// FIXME: We should either use fake systemd, or somehow enable it.
	//        Ref: https://github.com/scylladb/scylla-operator/issues/1379
	_, err = sc.conn.GetAllPropertiesContext(ctx, "systemd1.service")
	if err != nil {
		var godbusErr godbus.Error
		if errors.As(err, &godbusErr) && godbusErr.Name == "org.freedesktop.DBus.Error.ServiceUnknown" {
			t.Skip("systemd is not available, skipping the test")
		}
		t.Fatal(err)
	}

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
