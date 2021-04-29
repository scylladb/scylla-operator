// Copyright (C) 2021 ScyllaDB

package releasenotes

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"k8s.io/klog"
)

func isMergeCommit(commit *object.Commit) bool {
	return commit.NumParents() > 1
}

var (
	pullRequestNumberRe = regexp.MustCompile(`^Merge pull request #(\d+) .*`)
)

// Merge commit originating from merged PR has following format:
// 'Merge pull request #<pr-number> from <author>/<branch>'
func parsePullRequestNumber(message string) (number int, ok bool) {
	firstLine := strings.SplitN(message, "\n", 1)[0]
	m := pullRequestNumberRe.FindStringSubmatch(firstLine)
	if m == nil {
		return 0, false
	}
	number, err := strconv.Atoi(m[1])
	if err != nil {
		klog.Errorf("Can't parse pull request number from %q commit message: %s", firstLine, err)
		return 0, false
	}
	return number, true
}

func commitFromRevision(r *git.Repository, revName string) (*object.Commit, error) {
	rev, err := r.ResolveRevision(plumbing.Revision(revName))
	if err != nil {
		return nil, fmt.Errorf("resolve revision %q: %w", revName, err)
	}

	hash := plumbing.NewHash(rev.String())
	commit, err := r.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("commit object: %w", err)
	}

	return commit, nil
}

func listCommitsBetween(repositoryPath, startRef, endRef string) ([]*object.Commit, error) {
	r, err := git.PlainOpen(repositoryPath)
	if err != nil {
		return nil, err
	}

	startCommit, err := commitFromRevision(r, startRef)
	if err != nil {
		return nil, err
	}

	endCommit, err := commitFromRevision(r, endRef)
	if err != nil {
		return nil, err
	}

	mergeBases, err := endCommit.MergeBase(startCommit)
	if err != nil {
		return nil, fmt.Errorf("merge base: %w", err)
	}
	if mergeBases[0].Hash.String() != startCommit.Hash.String() {
		return nil, errors.New("start ref is not direct predecessor of end ref")
	}
	if len(mergeBases) != 1 {
		return nil, fmt.Errorf("ref ranges containing %d merge bases are not supported", len(mergeBases))
	}

	commitIter, err := r.Log(&git.LogOptions{From: endCommit.Hash})
	if err != nil {
		return nil, err
	}
	defer commitIter.Close()

	var commitsBetweenTags []*object.Commit
	if err := commitIter.ForEach(func(commit *object.Commit) error {
		if startCommit.Hash == commit.Hash {
			return storer.ErrStop
		}

		commitsBetweenTags = append(commitsBetweenTags, commit)
		return nil
	}); err != nil {
		return nil, err
	}

	return commitsBetweenTags, nil
}
