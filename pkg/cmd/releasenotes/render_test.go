// Copyright (C) 2021 ScyllaDB

package releasenotes

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	githubql "github.com/shurcooL/githubv4"
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

func makePR(number int, labels []string) *PullRequest {
	pr := &PullRequest{
		Number: githubql.Int(number),
	}

	for _, label := range labels {
		pr.Labels.Nodes = append(pr.Labels.Nodes, struct {
			Name githubql.String
		}{
			Name: githubql.String(label),
		})
	}

	return pr
}

func TestCategorizePullRequests(t *testing.T) {
	tt := []struct {
		name             string
		prs              []*PullRequest
		expectedSections []section
	}{
		{
			name: "no PRs return empty sections",
			prs:  nil,
			expectedSections: []section{
				{
					Title: "API Change",
				},
				{
					Title: "Feature",
				},
				{
					Title: "Bug",
				},
				{
					Title: "Documentation",
				},
				{
					Title: "Flaky/Failing Test",
				},
				{
					Title: "Other",
				},
				{
					Title: "Uncategorized",
				},
			},
		},
		{
			name: "multiple PRs with the same kind lands in the same category",
			prs: []*PullRequest{
				makePR(1, nil),
				makePR(2, []string{"priority/important-soon"}),
				makePR(3, []string{"kind/feature"}),
				makePR(4, []string{"kind/feature"}),
				makePR(5, []string{"kind/bug"}),
				makePR(6, []string{"kind/bug"}),
				makePR(7, []string{"kind/api-change"}),
				makePR(8, []string{"kind/api-change"}),
				makePR(9, []string{"kind/documentation"}),
				makePR(10, []string{"kind/documentation"}),
				makePR(11, []string{"kind/failing-test"}),
				makePR(12, []string{"kind/flake"}),
				makePR(13, []string{"kind/foo"}),
				makePR(14, []string{"kind/foo"}),
			},
			expectedSections: []section{
				{
					Title: "API Change",
					PullRequests: []*PullRequest{
						makePR(7, []string{"kind/api-change"}),
						makePR(8, []string{"kind/api-change"}),
					},
				},
				{
					Title: "Feature",
					PullRequests: []*PullRequest{
						makePR(3, []string{"kind/feature"}),
						makePR(4, []string{"kind/feature"}),
					},
				},
				{
					Title: "Bug",
					PullRequests: []*PullRequest{
						makePR(5, []string{"kind/bug"}),
						makePR(6, []string{"kind/bug"}),
					},
				},
				{
					Title: "Documentation",
					PullRequests: []*PullRequest{
						makePR(9, []string{"kind/documentation"}),
						makePR(10, []string{"kind/documentation"}),
					},
				},
				{
					Title: "Flaky/Failing Test",
					PullRequests: []*PullRequest{
						makePR(11, []string{"kind/failing-test"}),
						makePR(12, []string{"kind/flake"}),
					},
				},
				{
					Title: "Other",
					PullRequests: []*PullRequest{
						makePR(13, []string{"kind/foo"}),
						makePR(14, []string{"kind/foo"}),
					},
				},
				{
					Title: "Uncategorized",
					PullRequests: []*PullRequest{
						makePR(1, nil),
						makePR(2, []string{"priority/important-soon"}),
					},
				},
			},
		},
		{
			name: "precedences",
			prs: []*PullRequest{
				makePR(1, []string{
					"priority/important-soon",
					"kind/foo",
					"kind/flake",
					"kind/failing-test",
					"kind/documentation",
					"kind/bug",
					"kind/feature",
					"kind/api-change",
				}),
				makePR(2, []string{
					"priority/important-soon",
					"kind/foo",
					"kind/flake",
					"kind/failing-test",
					"kind/documentation",
					"kind/bug",
					"kind/feature",
				}),
				makePR(3, []string{
					"priority/important-soon",
					"kind/foo",
					"kind/flake",
					"kind/failing-test",
					"kind/documentation",
					"kind/bug",
				}),
				makePR(4, []string{
					"priority/important-soon",
					"kind/foo",
					"kind/flake",
					"kind/failing-test",
					"kind/documentation",
				}),
				makePR(5, []string{
					"priority/important-soon",
					"kind/foo",
					"kind/flake",
					"kind/failing-test",
				}),
				makePR(6, []string{
					"priority/important-soon",
					"kind/foo",
					"kind/flake",
				}),
				makePR(7, []string{
					"priority/important-soon",
					"kind/foo",
				}),
				makePR(8, []string{
					"priority/important-soon",
				}),
				makePR(9, nil),
			},
			expectedSections: []section{
				{
					Title: "API Change",
					PullRequests: []*PullRequest{
						makePR(1, []string{
							"priority/important-soon",
							"kind/foo",
							"kind/flake",
							"kind/failing-test",
							"kind/documentation",
							"kind/bug",
							"kind/feature",
							"kind/api-change",
						}),
					},
				},
				{
					Title: "Feature",
					PullRequests: []*PullRequest{
						makePR(2, []string{
							"priority/important-soon",
							"kind/foo",
							"kind/flake",
							"kind/failing-test",
							"kind/documentation",
							"kind/bug",
							"kind/feature",
						}),
					},
				},
				{
					Title: "Bug",
					PullRequests: []*PullRequest{
						makePR(3, []string{
							"priority/important-soon",
							"kind/foo",
							"kind/flake",
							"kind/failing-test",
							"kind/documentation",
							"kind/bug",
						}),
					},
				},
				{
					Title: "Documentation",
					PullRequests: []*PullRequest{
						makePR(4, []string{
							"priority/important-soon",
							"kind/foo",
							"kind/flake",
							"kind/failing-test",
							"kind/documentation",
						}),
					},
				},
				{
					Title: "Flaky/Failing Test",
					PullRequests: []*PullRequest{
						makePR(5, []string{
							"priority/important-soon",
							"kind/foo",
							"kind/flake",
							"kind/failing-test",
						}),
						makePR(6, []string{
							"priority/important-soon",
							"kind/foo",
							"kind/flake",
						}),
					},
				},
				{
					Title: "Other",
					PullRequests: []*PullRequest{
						makePR(7, []string{
							"priority/important-soon",
							"kind/foo",
						}),
					},
				},
				{
					Title: "Uncategorized",
					PullRequests: []*PullRequest{
						makePR(8, []string{
							"priority/important-soon",
						}),
						makePR(9, nil),
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := categorizePullRequests(tc.prs)

			if !reflect.DeepEqual(got, tc.expectedSections) {
				t.Errorf("expected and got sections differ: %s", cmp.Diff(tc.expectedSections, got))
			}
		})
	}
}
