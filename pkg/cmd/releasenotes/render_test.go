// Copyright (C) 2021 ScyllaDB

package releasenotes

import (
	"testing"
)

func TestCleanupPRTitle(t *testing.T) {
	ts := []struct {
		Name          string
		Title         string
		ExpectedTitle string
	}{
		{
			Name:          "Regular PR title",
			Title:         "Add XYZ feature",
			ExpectedTitle: "Add XYZ feature",
		},
		{
			Name:          "Branch prefix is removed",
			Title:         "[Anything] Add XYZ feature",
			ExpectedTitle: "Add XYZ feature",
		},
		{
			Name:          "whitespaces are trimmed",
			Title:         "       Add XYZ feature    \r\n",
			ExpectedTitle: "Add XYZ feature",
		},
	}

	for i := range ts {
		test := ts[i]
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			out, err := cleanupPRTitle(test.Title)
			if err != nil {
				t.Fatal(err)
			}

			if out != test.ExpectedTitle {
				t.Errorf("expected title %q, got %q", test.ExpectedTitle, out)
			}
		})
	}
}
