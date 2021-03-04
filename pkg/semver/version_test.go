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
	"testing"

	"github.com/blang/semver"
)

func TestSupportFeature(t *testing.T) {
	recentVersion := NewScyllaVersion("99.0.0")
	oldVersion := NewScyllaVersion("0.0.0")
	badVersion := NewScyllaVersion("foobar")

	fakeFeature := semver.MustParse("4.2.0")

	if !recentVersion.SupportFeatureUnsafe(fakeFeature) {
		t.Errorf("Recent version should support a previous version")
	}
	if oldVersion.SupportFeatureUnsafe(fakeFeature) {
		t.Errorf("Old version should not support a future version")
	}
	if !badVersion.SupportFeatureUnsafe(fakeFeature) {
		t.Errorf("Unkown version should support any version when unsafe")
	}

	if !recentVersion.SupportFeatureSafe(fakeFeature) {
		t.Errorf("Recent version should support a previous version")
	}
	if oldVersion.SupportFeatureSafe(fakeFeature) {
		t.Errorf("Recent version should support a previous version")
	}
	if badVersion.SupportFeatureSafe(fakeFeature) {
		t.Errorf("Unkown version should not support any version when safe")
	}
}
