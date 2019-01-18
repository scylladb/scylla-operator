package config

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMergeYAMLs(t *testing.T) {
	tests := []struct {
		initial     []byte
		override    []byte
		result      []byte
		expectedErr bool
	}{
		{
			[]byte("key: value"),
			[]byte("key: override_value"),
			[]byte("key: override_value\n"),
			false,
		},
		{
			[]byte("#comment"),
			[]byte("key: value"),
			[]byte("key: value\n"),
			false,
		},
		{
			[]byte("key: value"),
			[]byte("#comment"),
			[]byte("key: value\n"),
			false,
		},
		{
			[]byte("key1:\n  nestedkey1: nestedvalue1"),
			[]byte("key1:\n  nestedkey1: nestedvalue2"),
			[]byte("key1:\n  nestedkey1: nestedvalue2\n"),
			false,
		},
	}

	for _, test := range tests {
		result, err := mergeYAMLs(test.initial, test.override)
		if !bytes.Equal(result, test.result) {
			t.Errorf("Merge of '%s' and '%s' was incorrect,\n got: %s,\n want: %s.",
				test.initial, test.override, result, test.result)
		}
		if err == nil && test.expectedErr {
			t.Errorf("Expected error.")
		}
		if err != nil && !test.expectedErr {
			t.Logf("Got an error as expected: %s", err.Error())
		}
	}
}

func TestAllowedCPUs(t *testing.T) {
	cpusAllowed, err := getCPUsAllowedList("./procstatus")
	require.Equal(t, err, nil)
	t.Log(cpusAllowed)
}
