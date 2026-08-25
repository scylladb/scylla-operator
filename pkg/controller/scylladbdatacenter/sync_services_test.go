package scylladbdatacenter

import (
	"testing"
)

func Test_getMemberServiceOrdinal(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                string
		memberServiceName   string
		expectedOrdinal     int32
		expectedErrorString string
	}{
		{
			name:              "single digit ordinal",
			memberServiceName: "basic-dc-a-1",
			expectedOrdinal:   1,
		},
		{
			name:              "multi digit ordinal",
			memberServiceName: "basic-dc-a-42",
			expectedOrdinal:   42,
		},
		{
			name:                "name without an ordinal",
			memberServiceName:   "basic-dc-a",
			expectedErrorString: `can't parse ordinal from member service name "basic-dc-a"`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ordinal, err := getMemberServiceOrdinal(tc.memberServiceName)

			gotErrorString := ""
			if err != nil {
				gotErrorString = err.Error()
			}
			if gotErrorString != tc.expectedErrorString {
				t.Fatalf("expected error %q, got %q", tc.expectedErrorString, gotErrorString)
			}

			if ordinal != tc.expectedOrdinal {
				t.Errorf("expected ordinal %d, got %d", tc.expectedOrdinal, ordinal)
			}
		})
	}
}
