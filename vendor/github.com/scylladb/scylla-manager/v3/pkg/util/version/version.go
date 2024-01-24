// Copyright (C) 2017 ScyllaDB

package version

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
)

// Short excludes any metadata or pre-release information. For example,
// for a version "1.2.3-20200101.b41b3dbs1b", it will return "1.2.3".
// If provided string isn't version, it will return input string.
func Short(v string) string {
	ver, err := version.NewVersion(v)
	if err != nil {
		return v
	}

	parts := make([]string, len(ver.Segments()))
	for i, s := range ver.Segments() {
		parts[i] = fmt.Sprint(s)
	}

	return strings.Join(parts, ".")
}

const (
	masterMajorVersion           = "666"
	masterEnterpriseMajorVersion = "9999"

	masterVersionSuffix           = ".dev"
	masterVersionLongSuffix       = ".development"
	masterEnterpriseVersionSuffix = ".enterprise_dev"

	masterVersion                 = masterMajorVersion + masterVersionSuffix
	masterLongVersion             = masterMajorVersion + masterVersionLongSuffix
	masterEnterpriseMasterVersion = masterEnterpriseMajorVersion + masterEnterpriseVersionSuffix
)

// MasterVersion returns whether provided version string originates from master branch.
func MasterVersion(v string) bool {
	v = strings.Split(v, "-")[0]
	return v == masterLongVersion || v == masterEnterpriseMasterVersion || v == masterVersion
}

// TrimMaster returns version string without master branch bloat breaking semantic
// format constraints.
func TrimMaster(v string) string {
	if v == "Snapshot" {
		return masterMajorVersion
	}

	v = strings.Split(v, "-")[0]
	v = strings.TrimSuffix(v, masterVersionSuffix)
	v = strings.TrimSuffix(v, masterVersionLongSuffix)
	v = strings.TrimSuffix(v, masterEnterpriseVersionSuffix)

	return v
}

var releaseCandidateRe = regexp.MustCompile(`\.rc([0-9]+)`)

// TransformReleaseCandidate replaces `.rcX` in version string with `~` instead
// of the dot.
func TransformReleaseCandidate(v string) string {
	v = releaseCandidateRe.ReplaceAllString(v, `~rc$1`)

	return v
}

var verRe = regexp.MustCompile(`^[0-9]+\.[0-9]+(\.[0-9]+)?`)

// CheckConstraint returns whether version fulfills given constraint.
func CheckConstraint(ver, constraint string) (bool, error) {
	if verRe.FindString(ver) == "" {
		return false, errors.Errorf("Unsupported Scylla version: %s", ver)
	}

	// Extract only version number
	ver = verRe.FindString(ver)

	v, err := version.NewSemver(ver)
	if err != nil {
		return false, err
	}

	c, err := version.NewConstraint(constraint)
	if err != nil {
		return false, errors.Errorf("version constraint syntax error: %s", err)
	}
	return c.Check(v), nil
}
