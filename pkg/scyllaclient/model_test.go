// Copyright (c) 2026 ScyllaDB.

package scyllaclient

import (
	"testing"
)

func TestOperationalModeFromString(t *testing.T) {
	t.Parallel()

	tt := []struct {
		str      string
		expected OperationalMode
	}{
		// NONE is reported as STARTING.
		{str: "STARTING", expected: OperationalModeStarting},
		{str: "JOINING", expected: OperationalModeJoining},
		{str: "BOOTSTRAP", expected: OperationalModeBootstrap},
		{str: "NORMAL", expected: OperationalModeNormal},
		{str: "LEAVING", expected: OperationalModeLeaving},
		{str: "DECOMMISSIONED", expected: OperationalModeDecommissioned},
		{str: "MOVING", expected: OperationalModeMoving},
		{str: "DRAINING", expected: OperationalModeDraining},
		{str: "DRAINED", expected: OperationalModeDrained},
		{str: "MAINTENANCE", expected: OperationalModeMaintenance},
		{str: "", expected: OperationalModeUnknown},
	}

	for _, tc := range tt {
		t.Run(tc.str, func(t *testing.T) {
			t.Parallel()

			got := operationalModeFromString(tc.str)
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
			if got.String() != string(tc.expected) {
				t.Errorf("expected String() %q, got %q", tc.expected, got.String())
			}
		})
	}
}
