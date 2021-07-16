// Copyright (C) 2021 ScyllaDB

package releasenotes

import (
	"context"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (o *GenerateOptions) Run(ctx context.Context) error {
	commits, err := listCommitsBetween(o.RepositoryPath, o.StartRef, o.EndRef)
	if err != nil {
		return err
	}

	var requiredPullRequestsNumbers []int
	for _, c := range commits {
		if !isMergeCommit(c) {
			continue
		}

		n, ok := parsePullRequestNumber(c.Message)
		if !ok {
			klog.Errorf("Can't parse pull request number from %q commit", c.Hash)
			continue
		}

		requiredPullRequestsNumbers = append(requiredPullRequestsNumbers, n)
	}

	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Committer.When.Before(commits[j].Committer.When)
	})
	startDate, endDate := commits[0].Committer.When, commits[len(commits)-1].Committer.When

	prs, err := listPullRequests(ctx, o.ghClient, o.OrganizationName, o.RepositoryName, startDate, endDate)
	if err != nil {
		return err
	}
	numberToPR := map[int]*PullRequest{}
	for i := range prs {
		numberToPR[int(prs[i].Number)] = &prs[i]
	}

	var filteredPRs []PullRequest
	var errs []error
	for _, prNumber := range requiredPullRequestsNumbers {
		pr, found := numberToPR[prNumber]
		if !found {
			errs = append(errs, fmt.Errorf("PR #%d not found in GitHub", prNumber))
			continue
		}

		filteredPRs = append(filteredPRs, *pr)
	}
	if errs != nil {
		return errors.NewAggregate(errs)
	}

	if err := renderReleaseNotes(o.Out, o.ReleaseName, o.PreviousReleaseName, filteredPRs); err != nil {
		return err
	}

	return nil
}
