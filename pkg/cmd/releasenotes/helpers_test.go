// Copyright (C) 2021 ScyllaDB

package releasenotes

import (
	"testing"
)

func TestParsePullRequestNumber(t *testing.T) {
	ts := []struct {
		Name            string
		Message         string
		ExpectedSuccess bool
		ExpectedNumber  int
	}{
		{
			Name:            "Not a PR merge commit",
			Message:         "Added XYZ feature",
			ExpectedSuccess: false,
		},
		{
			Name:            "PR merge commit",
			Message:         "Merge pull request #571 from author/some/thing\nBlablablabalh",
			ExpectedSuccess: true,
			ExpectedNumber:  571,
		},
		{
			Name:            "Revert of PR merge commit",
			Message:         `Revert "Merge pull request #571 from author/some/thing\"`,
			ExpectedSuccess: false,
		},
		{
			Name:            "Invalid PR message",
			Message:         "Merge pull request #foo from author/some/thing\nBlablablabalh",
			ExpectedSuccess: false,
		},
	}

	for _, test := range ts {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			number, ok := parsePullRequestNumber(test.Message)
			if test.ExpectedSuccess != ok {
				t.Errorf("Expected success: %v, got result: %v", test.ExpectedSuccess, ok)
			}
			if number != test.ExpectedNumber {
				t.Errorf("Expected PR number %d, got %d", test.ExpectedNumber, number)
			}
		})
	}
}
