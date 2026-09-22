// Copyright (c) 2026 ScyllaDB.

package controllermanager

import (
	"errors"
	"testing"
)

func TestErrorCollector(t *testing.T) {
	t.Parallel()

	errs := &errorCollector{}
	if err := errs.Err(); err != nil {
		t.Fatalf("expected no error before any call, got %v", err)
	}

	first := errors.New("first")
	second := errors.New("second")
	one := collectError(errs, func() (int, error) { return 1, nil })
	zero := collectError(errs, func() (int, error) { return 0, first })
	text := collectError(errs, func() (string, error) { return "", second })

	if one != 1 || zero != 0 || text != "" {
		t.Errorf("expected the values of the calls to be returned as they are, got %d, %d, %q", one, zero, text)
	}
	err := errs.Err()
	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Errorf("expected the aggregate to carry both errors, got %v", err)
	}
}
