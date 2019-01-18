// Copyright 2018 The Jetstack Navigator Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package version

import (
	"encoding/json"
	"strconv"

	semver "github.com/hashicorp/go-version"
)

// Version represents a Cassandra database server version.
// Cassandra does not adhere to semver.
// A Cassandra version string may omit the patch version.
// In Navigator we query the version of the running Cassandra service via the JMX interface.
// It is returned as a string via:
// StorageService.getReleaseVersion: https://github.com/apache/cassandra/blob/cassandra-3.11.2/src/java/org/apache/cassandra/service/StorageService.java#L2863
// FBUtilities.getReleaseVersionString: https://github.com/apache/cassandra/blob/cassandra-3.11.2/src/java/org/apache/cassandra/utils/FBUtilities.java#L326
// Which appears to read the version string from a `Properties` API which appears to be created via this XML file: https://github.com/apache/cassandra/blob/cassandra-3.11.2/build.xml#L790
// Internally, Cassandra converts the version string to a `CassandraVersion` object which supports rich comparison.
// See https://github.com/apache/cassandra/blob/cassandra-3.11.2/src/java/org/apache/cassandra/utils/CassandraVersion.java
// In Navigator we parse the Cassandra version string as early as possible, into a similar Cassandra Version object.
// This also fixes the missing Patch number and stores the version internally as a semver.
// It also keeps a reference to the original version string so that we can report that in our API.
// So that the version reported in our API matches the version that an administrator expects.
//
// +k8s:deepcopy-gen=true
type Version struct {
	versionString string
	semver        *semver.Version
}

func New(s string) *Version {
	v := &Version{}
	err := v.set(s)
	if err != nil {
		panic(err)
	}
	return v
}

func (v *Version) set(s string) error {
	sv, err := semver.NewVersion(s)
	if err != nil {
		return err
	}
	v.versionString = s
	v.semver = sv
	return nil
}

func (v *Version) Equal(versionB *Version) bool {
	return v.semver.Equal(versionB.semver)
}

func (v Version) String() string {
	return v.versionString
}

func (v *Version) Semver() *semver.Version {
	return v.semver
}

func (v *Version) UnmarshalJSON(data []byte) error {
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}
	return v.set(s)
}

var _ json.Unmarshaler = &Version{}

func (v Version) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(v.String())), nil
}

var _ json.Marshaler = &Version{}

// DeepCopy returns a deep-copy of the Version value.
// If the underlying semver is a nil pointer, assume that the zero value is being copied,
// and return that.
func (v Version) DeepCopy() Version {
	if v.semver == nil {
		return Version{}
	}
	return *New(v.String())
}
