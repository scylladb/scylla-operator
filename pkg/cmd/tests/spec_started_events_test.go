// Copyright (c) 2026 ScyllaDB.

package tests

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSpecStartedReceiver(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                string
		write               string
		expectedEvents      []specStartedEvent
		expectedPassthrough string
	}{
		{
			name:  "event is handled",
			write: `{"kind":"scylla-operator-tests/spec-started","process":3,"text":"ScyllaCluster should scale","location":"github.com/scylladb/scylla-operator/test/e2e/set/scyllacluster/scyllacluster_scale.go:42","labels":["SuiteKindFast"]}`,
			expectedEvents: []specStartedEvent{
				{
					Kind:     specStartedEventKind,
					Process:  3,
					Text:     "ScyllaCluster should scale",
					Location: "github.com/scylladb/scylla-operator/test/e2e/set/scyllacluster/scyllacluster_scale.go:42",
					Labels:   []string{"SuiteKindFast"},
				},
			},
		},
		{
			name:                "plain output is passed through",
			write:               "partial output",
			expectedPassthrough: "partial output",
		},
		{
			name:                "JSON of another kind is passed through",
			write:               `{"kind":"something-else","process":3}`,
			expectedPassthrough: `{"kind":"something-else","process":3}`,
		},
		{
			name:                "invalid JSON is passed through",
			write:               `{"kind":"scylla-operator-tests/spec-started"`,
			expectedPassthrough: `{"kind":"scylla-operator-tests/spec-started"`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var events []specStartedEvent
			var passthrough []byte
			receiver := newSpecStartedReceiver(
				func(event specStartedEvent) {
					events = append(events, event)
				},
				func(p []byte) (int, error) {
					passthrough = append(passthrough, p...)
					return len(p), nil
				},
			)

			n, err := receiver.Write([]byte(tc.write))
			if err != nil {
				t.Fatal(err)
			}
			if n != len(tc.write) {
				t.Errorf("expected to write %d bytes, got %d", len(tc.write), n)
			}

			if diff := cmp.Diff(tc.expectedEvents, events); diff != "" {
				t.Errorf("expected and got events differ:\n%s", diff)
			}
			if diff := cmp.Diff(tc.expectedPassthrough, string(passthrough)); diff != "" {
				t.Errorf("expected and got pass-through differ:\n%s", diff)
			}
		})
	}
}
