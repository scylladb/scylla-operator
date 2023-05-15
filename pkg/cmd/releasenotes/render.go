// Copyright (C) 2021 ScyllaDB

package releasenotes

import (
	"fmt"
	"html/template"
	"io"
	"regexp"
	"sort"
	"strings"

	"k8s.io/klog/v2"
)

const releaseNotesTemplate = `
# Release notes for {{ .Release }}
## Container images

` + "```\n{{ .ContainerImageName }}:{{ .Release }}\n```" + `

## Changes By Kind (since {{ .PreviousRelease }})

{{- range $section := .Sections }}
  {{- if $section.PullRequests }}

### {{ $section.Title }}

    {{- range $pr := $section.PullRequests }}
{{ FormatPR $pr }}
    {{- end }}
  {{- end }}
{{- end }}

---
`

var branchPrefixRe = regexp.MustCompile(`^(\[[^]]+])?(.*)`)

func cleanupPRTitle(title string) (string, error) {
	m := branchPrefixRe.FindStringSubmatch(strings.TrimSpace(title))
	if len(m) != 3 {
		return "", fmt.Errorf("can't parse PR title %q", title)
	}
	return strings.TrimSpace(m[2]), nil
}

func prURL(pr PullRequest) string {
	return fmt.Sprintf("https://github.com/%s/%s/pull/%d", pr.Repository.Owner.Login, pr.Repository.Name, pr.Number)
}

func userURL(login string) string {
	return fmt.Sprintf("https://github.com/%s", login)
}

func formatPR(pr PullRequest) (string, error) {
	title, err := cleanupPRTitle(string(pr.Title))
	if err != nil {
		return "", err
	}
	userLogin := pr.Author.Login
	details := fmt.Sprintf("[#%d](%s),[@%s](%s)", pr.Number, prURL(pr), userLogin, userURL(string(userLogin)))
	return fmt.Sprintf("* %s (%s)", title, details), nil
}

type section struct {
	Title        string
	PullRequests []*PullRequest
}

func categorizePullRequests(prs []*PullRequest) []section {
	remaining := map[int]*PullRequest{}
	for i, pr := range prs {
		remaining[i] = pr
	}

	// Given the sections are extracted from the map, the following list has to be ordered by priority
	// in case there is more than one "kind/" label present.
	sections := []section{
		{"API Change", extract(remaining, byKinds("api-change"))},
		{"Feature", extract(remaining, byKinds("feature"))},
		{"Bug", extract(remaining, byKinds("bug"))},
		{"Documentation", extract(remaining, byKinds("documentation"))},
		{"Flaky/Failing Test", extract(remaining, byKinds("failing-test", "flake"))},
		{"Other", extract(remaining, anyKind)},
		{"Uncategorized", extract(remaining, missingKind)},
	}

	// The sets are mutually exclusive and there shall be no PR left.
	if len(remaining) != 0 {
		klog.Fatalf("Internal error: %d unmatched PRs: %#v", len(remaining), remaining)
	}

	for _, section := range sections {
		// Sort sections to have a deterministic output.
		sort.Slice(section.PullRequests, func(i, j int) bool {
			return section.PullRequests[i].Number < section.PullRequests[j].Number
		})
	}

	return sections
}

type releaseNoteData struct {
	Sections           []section
	Release            string
	PreviousRelease    string
	ContainerImageName string
}

func renderReleaseNotes(out io.Writer, containerImageName, release, previousRelease string, pullRequests []*PullRequest) error {
	// Render in the order of merge date.
	sort.Slice(pullRequests, func(i, j int) bool {
		return pullRequests[i].MergedAt.Before(pullRequests[j].MergedAt.Time)
	})

	sections := categorizePullRequests(pullRequests)

	data := releaseNoteData{
		Release:            release,
		PreviousRelease:    previousRelease,
		Sections:           sections,
		ContainerImageName: containerImageName,
	}

	t := template.New("release-notes").Funcs(map[string]interface{}{
		"FormatPR": formatPR,
	})
	t, err := t.Parse(releaseNotesTemplate)
	if err != nil {
		return err
	}

	if err := t.Execute(out, data); err != nil {
		return err
	}

	return nil
}

func extract(prs map[int]*PullRequest, match func(*PullRequest) bool) []*PullRequest {
	var res []*PullRequest

	for key, pr := range prs {
		if match(pr) {
			res = append(res, pr)
			delete(prs, key)
		}
	}

	return res
}

func byKinds(kinds ...string) func(*PullRequest) bool {
	return func(pr *PullRequest) bool {
		for _, label := range pr.Labels.Nodes {
			for _, kind := range kinds {
				if string(label.Name) == "kind/"+kind {
					return true
				}
			}
		}
		return false
	}
}

func missingKind(pr *PullRequest) bool {
	for _, label := range pr.Labels.Nodes {
		if strings.HasPrefix(string(label.Name), "kind/") {
			return false
		}
	}
	return true
}

func anyKind(pr *PullRequest) bool {
	return !missingKind(pr)
}
