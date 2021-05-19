// Copyright (C) 2021 ScyllaDB

package releasenotes

import (
	"fmt"
	"html/template"
	"io"
	"regexp"
	"sort"
	"strings"
)

const releaseNotesTemplate = `
# Release notes for {{ .Release }}
## Container images

` + "```\ndocker.io/scylladb/scylla-operator:{{ .Release }}\n```" + `

## Changes By Kind

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
	PullRequests []PullRequest
}

type releaseNoteData struct {
	Sections []section
	Release  string
}

func renderReleaseNotes(out io.Writer, release string, pullRequests []PullRequest) error {
	// Render in the order of merge date.
	sort.Slice(pullRequests, func(i, j int) bool {
		return pullRequests[i].MergedAt.Before(pullRequests[j].MergedAt.Time)
	})

	prsByKind, uncategorized := splitByKind(pullRequests)

	sections := []section{
		{"API Change", extractByKind(prsByKind, "api-change")},
		{"Feature", extractByKind(prsByKind, "feature")},
		{"Bug", extractByKind(prsByKind, "bug")},
		{"Documentation", extractByKind(prsByKind, "documentation")},
		{"Flaky/Failing Test", extractByKind(prsByKind, "failing-test", "flake")},
		{"Other", flatten(prsByKind)},
		{"Uncategorized", uncategorized},
	}

	data := releaseNoteData{
		Release:  release,
		Sections: sections,
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

func flatten(m map[string][]PullRequest) []PullRequest {
	var prs []PullRequest
	for _, v := range m {
		prs = append(prs, v...)
	}
	return prs
}

func extractByKind(prsByKind map[string][]PullRequest, kinds ...string) []PullRequest {
	var prs []PullRequest
	for _, kind := range kinds {
		prs = append(prs, prsByKind[kind]...)
		delete(prsByKind, kind)
	}
	return prs
}

func prKind(pr PullRequest) (string, bool) {
	for _, label := range pr.Labels.Nodes {
		parts := strings.Split(string(label.Name), "/")
		if len(parts) == 2 && parts[0] == "kind" {
			return parts[1], true
		}
	}
	return "", false
}

func splitByKind(prs []PullRequest) (map[string][]PullRequest, []PullRequest) {
	prsByKind := make(map[string][]PullRequest)
	var uncategorized []PullRequest

	for _, pr := range prs {
		kind, hasKind := prKind(pr)
		if hasKind {
			prsByKind[kind] = append(prsByKind[kind], pr)
		} else {
			uncategorized = append(uncategorized, pr)
		}
	}
	return prsByKind, uncategorized
}
