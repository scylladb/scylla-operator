// Copyright (C) 2021 ScyllaDB

package releasenotes

import (
	"context"
	"fmt"
	"strings"
	"time"

	githubql "github.com/shurcooL/githubv4"
	"k8s.io/klog/v2"
)

// PullRequest holds graphql data about a PR, including its commits and their contexts.
type PullRequest struct {
	Number githubql.Int
	Author struct {
		Login githubql.String
	}
	Title  githubql.String
	Labels struct {
		Nodes []struct {
			Name githubql.String
		}
	} `graphql:"labels(first: 100)"`
	Repository struct {
		Name          githubql.String
		NameWithOwner githubql.String
		Owner         struct {
			Login githubql.String
		}
	}
	MergedAt githubql.DateTime
}

type PRNode struct {
	PullRequest PullRequest `graphql:"... on PullRequest"`
}

type searchQuery struct {
	RateLimit struct {
		Cost      githubql.Int
		Remaining githubql.Int
	}
	Search struct {
		PageInfo struct {
			HasNextPage githubql.Boolean
			EndCursor   githubql.String
		}
		Nodes []PRNode
	} `graphql:"search(type: ISSUE, first: 100, after: $searchCursor, query: $query)"`
}

func dateToken(start, end time.Time) string {
	// GitHub's GraphQL API silently fails if you provide it with an invalid time
	// string.
	// Dates before 1970 (unix epoch) are considered invalid.
	startString, endString := "*", "*"
	if start.Year() >= 1970 {
		// Make sure we list all the PR, GitHub ranges are exclusive.
		startString = start.Add(-1 * time.Second).UTC().Format(time.RFC3339Nano)
	} else {
		klog.Warningf("Search start date is before year 1970, using 'any' filter instead")
	}
	if end.Year() >= 1970 {
		// Make sure we list all the PR, GitHub ranges are exclusive.
		endString = end.Add(1 * time.Second).UTC().Format(time.RFC3339Nano)
	} else {
		klog.Warningf("Search end date is before year 1970, using 'any' filter instead")
	}
	return fmt.Sprintf("merged:%s..%s", startString, endString)
}

func repoToken(owner, name string) string {
	return fmt.Sprintf("repo:%s/%s", owner, name)
}

func listPullRequests(ctx context.Context, ghClient *githubql.Client, owner, repoName string, start, end time.Time) ([]PullRequest, error) {
	params := []string{"is:pr", "is:merged", repoToken(owner, repoName), dateToken(start, end)}

	var cursor *githubql.String
	var sq searchQuery

	vars := map[string]interface{}{
		"query":        githubql.String(strings.Join(params, " ")),
		"searchCursor": cursor,
	}
	var pullRequests []PullRequest
	for {
		klog.V(2).InfoS("Listing pull requests", "Vars", vars)
		if err := ghClient.Query(ctx, &sq, vars); err != nil {
			return nil, err
		}

		for _, n := range sq.Search.Nodes {
			pullRequests = append(pullRequests, n.PullRequest)
		}

		if !sq.Search.PageInfo.HasNextPage {
			break
		}

		cursor = &sq.Search.PageInfo.EndCursor
		vars["searchCursor"] = cursor
	}

	return pullRequests, nil
}
