package systemd

import (
	"context"
	"errors"
	"fmt"
	"testing"

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
