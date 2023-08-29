// Copyright (c) 2023 ScyllaDB.

package scyllafeatures

import (
	"fmt"

	"github.com/blang/semver"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
)

var (
	scyllaEnterpriseMinimalVersion = semver.MustParse("2000.0.0")
)

type ScyllaFeature string

const (
	ReplacingNodeUsingHostID                          ScyllaFeature = "ReplacingNodeUsingHostID"
	ExposingScyllaClusterViaServiceOtherThanClusterIP ScyllaFeature = "ExposingScyllaClusterViaServiceOtherThanClusterIP"
)

type scyllaDBVersionMinimalConstraint struct {
	openSource semver.Version
	enterprise semver.Version
}

var featureMinimalVersionConstraints = map[ScyllaFeature]scyllaDBVersionMinimalConstraint{
	ReplacingNodeUsingHostID: {
		openSource: semver.MustParse("5.2.0"),
		enterprise: semver.MustParse("2023.1.0"),
	},
	// Exposing requires ReplacingNodeUsingHostID, so minimal values should be the same.
	ExposingScyllaClusterViaServiceOtherThanClusterIP: {
		openSource: semver.MustParse("5.2.0"),
		enterprise: semver.MustParse("2023.1.0"),
	},
}

func VersionSupports(version string, feature ScyllaFeature) (bool, error) {
	constraints, ok := featureMinimalVersionConstraints[feature]
	if !ok {
		return false, fmt.Errorf("unable to find minimal version constraints, unknown feature %q", feature)
	}

	parsedVersion, err := semver.Parse(version)
	if err != nil {
		return false, fmt.Errorf("can't parse ScyllaCluster version %q: %w", version, err)
	}

	if isOpenSource(parsedVersion) && parsedVersion.GTE(constraints.openSource) {
		return true, nil
	}

	if isEnterprise(parsedVersion) && parsedVersion.GTE(constraints.enterprise) {
		return true, nil
	}

	return false, nil
}

func Supports(sc *scyllav1.ScyllaCluster, feature ScyllaFeature) (bool, error) {
	return VersionSupports(sc.Spec.Version, feature)
}

func isEnterprise(v semver.Version) bool {
	return v.GTE(scyllaEnterpriseMinimalVersion)
}

func isOpenSource(v semver.Version) bool {
	return v.LT(scyllaEnterpriseMinimalVersion)
}
