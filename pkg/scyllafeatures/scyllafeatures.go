// Copyright (c) 2023 ScyllaDB.

package scyllafeatures

import (
	"fmt"

	"github.com/blang/semver"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
)

var (
	scyllaEnterpriseMinimalVersion = semver.MustParse("2000.0.0")
)

type ScyllaFeature string

const (
	ReplacingNodeUsingHostID ScyllaFeature = "ReplacingNodeUsingHostID"
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
}

func VersionSupports(image string, feature ScyllaFeature) (bool, error) {
	version, err := naming.ImageToVersion(image)
	if err != nil {
		return false, fmt.Errorf("can't get version from image %q: %w", image, err)
	}

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

func Supports(sdc *scyllav1alpha1.ScyllaDBDatacenter, feature ScyllaFeature) (bool, error) {
	return VersionSupports(sdc.Spec.ScyllaDB.Image, feature)
}

func isEnterprise(v semver.Version) bool {
	return v.GTE(scyllaEnterpriseMinimalVersion)
}

func isOpenSource(v semver.Version) bool {
	return v.LT(scyllaEnterpriseMinimalVersion)
}
