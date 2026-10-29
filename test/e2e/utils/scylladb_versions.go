// Copyright (C) 2026 ScyllaDB

package utils

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
)

// ListImageRepositoryTags lists the tags of the provided image repository.
func ListImageRepositoryTags(ctx context.Context, repository string) ([]string, error) {
	named, err := reference.ParseNormalizedNamed(repository)
	if err != nil {
		return nil, fmt.Errorf("invalid repository %q: %w", repository, err)
	}

	if !reference.IsNameOnly(named) {
		return nil, fmt.Errorf("repository %q must not have a tag or a digest", repository)
	}

	ref, err := docker.NewReference(reference.TagNameOnly(named))
	if err != nil {
		return nil, fmt.Errorf("can't create a reference for repository %q: %w", repository, err)
	}

	tags, err := docker.GetRepositoryTags(ctx, &types.SystemContext{}, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags of %q: %w", repository, err)
	}

	return tags, nil
}

// parseScyllaDBVersion parses a ScyllaDB release or release candidate tag. Other tags, like floating minors, aren't
// versions to update from.
func parseScyllaDBVersion(tag string) (semver.Version, bool) {
	// Semver compares the alphanumeric "rc10" lexically, below "rc9", so the candidate number is split off into
	// a numeric identifier.
	v, err := semver.Parse(strings.Replace(tag, "-rc", "-rc.", 1))
	if err != nil || len(v.Build) > 0 {
		return semver.Version{}, false
	}

	if len(v.Pre) > 0 && (len(v.Pre) != 2 || v.Pre[0].VersionStr != "rc" || !v.Pre[1].IsNum) {
		return semver.Version{}, false
	}

	return v, true
}

// parseScyllaDBTargetVersion parses the tag of the ScyllaDB image under test. On top of the tags parseScyllaDBVersion
// accepts, it takes the tags of packaged builds, like the nightly "2026.4.0-rc1-0.20261026.09174ad4a5a7", as their
// version, "2026.4.0-rc1", and development versions, like "2026.4.0-dev".
func parseScyllaDBTargetVersion(tag string) (semver.Version, bool) {
	// Packaged builds append the package release, "0.<date>.<commit>", and optionally the architecture.
	version, _, _ := strings.Cut(tag, "-0.")

	v, err := semver.Parse(version)
	if err == nil && len(v.Build) == 0 && len(v.Pre) == 1 && v.Pre[0].VersionStr == "dev" {
		return v, true
	}

	return parseScyllaDBVersion(version)
}

func isSameScyllaDBMinor(a, b semver.Version) bool {
	return a.Major == b.Major && a.Minor == b.Minor
}

func isLowerScyllaDBMinor(a, b semver.Version) bool {
	return a.Major < b.Major || (a.Major == b.Major && a.Minor < b.Minor)
}

// findHighestScyllaDBVersion returns the highest of the release and release candidate tags matching the predicate.
// It returns false when there is none, or when the target's version can't be parsed.
func findHighestScyllaDBVersion(tags []string, target string, matches func(v, target semver.Version) bool) (string, bool) {
	t, ok := parseScyllaDBTargetVersion(target)
	if !ok {
		return "", false
	}

	var highest *semver.Version
	highestTag := ""
	for _, tag := range tags {
		v, ok := parseScyllaDBVersion(tag)
		if !ok || !matches(v, t) {
			continue
		}

		if highest == nil || v.GT(*highest) {
			highest = &v
			highestTag = tag
		}
	}

	return highestTag, highest != nil
}

// FindScyllaDBUpdateFromVersion returns the highest of the tags lower than the target within the target's minor,
// release candidates included.
func FindScyllaDBUpdateFromVersion(tags []string, target string) (string, bool) {
	return findHighestScyllaDBVersion(tags, target, func(v, t semver.Version) bool {
		return isSameScyllaDBMinor(v, t) && v.LT(t)
	})
}

// FindScyllaDBUpgradeFromVersion returns the highest of the tags of the nearest lower minor than the target's, so
// a release candidate when the minor has no release yet.
func FindScyllaDBUpgradeFromVersion(tags []string, target string) (string, bool) {
	// Minors compare before patches, so the highest version of all lower minors is of the nearest one.
	return findHighestScyllaDBVersion(tags, target, func(v, t semver.Version) bool {
		return isLowerScyllaDBMinor(v, t)
	})
}
