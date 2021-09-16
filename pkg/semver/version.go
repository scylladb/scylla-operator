/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package semver

import (
	"github.com/blang/semver"
)

var (
	ScyllaVersionThatSupportsArgs              = semver.MustParse("4.2.0")
	ScyllaVersionThatSupportsDisablingIOTuning = semver.MustParse("4.3.0")
	//TODO: verify that
	ScyllaVersionThatSupportsDisablingWritebackCache = semver.MustParse("2021.0.0")
)

// ScyllaVersion contains the version of a cluster with unkown version support
type ScyllaVersion struct {
	version semver.Version
	unknown bool
}

func NewScyllaVersion(v string) ScyllaVersion {
	version, err := semver.Parse(v)
	if err != nil {
		return ScyllaVersion{unknown: true}
	}
	return ScyllaVersion{version: version, unknown: false}
}

// SupportFeatureUnsafe return true if a feature is supported (and always true if the version is unknown)
func (sv ScyllaVersion) SupportFeatureUnsafe(featureVersion semver.Version) bool {
	return sv.unknown || sv.version.GTE(featureVersion)
}

// SupportFeatureSafe return true if a feature is supported (and always false if the version is unknown)
func (sv ScyllaVersion) SupportFeatureSafe(featureVersion semver.Version) bool {
	return !sv.unknown && sv.version.GTE(featureVersion)
}
