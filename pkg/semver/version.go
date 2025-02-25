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
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/blang/semver"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"k8s.io/klog/v2"
)

type ImageVersionInfo struct {
	SemVersion  string
	fullVersion string
}

var (
	cacheLockScyllaDBImageVersionAndDigest sync.RWMutex
	cacheScyllaDBImageVersionAndDigest     = make(map[string]ImageVersionInfo)
)

var (
	ScyllaVersionThatSupportsDisablingWritebackCache = semver.MustParse("2021.0.0")
)

// ScyllaVersion contains the version of a cluster with unknown version support
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

func GetImageVersionAndDigest(imageName, version string) (string, string, error) {
	// If the version is already in <tag>@<digest> format, extract the semantic version
	if strings.Contains(version, "@") {
		semVersion, err := computeSemVersion(version)
		if err != nil {
			return "", "", err
		}
		return semVersion, version, nil
	}

	imageReference := fmt.Sprintf("scylladb/%s", imageName)
	if strings.Contains(version, ":") {
		imageReference = fmt.Sprintf("%s@%s", imageReference, version)
	} else {
		imageReference = fmt.Sprintf("%s:%s", imageReference, version)
	}

	cacheLockScyllaDBImageVersionAndDigest.RLock()
	cached, ok := cacheScyllaDBImageVersionAndDigest[imageReference]
	cacheLockScyllaDBImageVersionAndDigest.RUnlock()
	if ok {
		return cached.SemVersion, cached.fullVersion, nil
	}

	fullVersion, err := computeFullImageVersion(imageReference)
	if err != nil {
		return "", "", err
	}

	semVersion, err := computeSemVersion(fullVersion)
	if err != nil {
		return "", "", err
	}

	cacheLockScyllaDBImageVersionAndDigest.Lock()
	cacheScyllaDBImageVersionAndDigest[imageReference] = ImageVersionInfo{
		SemVersion:  semVersion,
		fullVersion: fullVersion,
	}
	cacheLockScyllaDBImageVersionAndDigest.Unlock()
	return semVersion, fullVersion, nil
}

func computeFullImageVersion(imageReference string) (string, error) {
	ref, err := name.ParseReference(imageReference)
	if err != nil {
		return "", fmt.Errorf("can't parse image reference: %w", err)
	}

	img, err := remote.Image(ref, remote.WithAuth(authn.Anonymous))
	if err != nil {
		return "", fmt.Errorf("can't fetch image: %w", err)
	}

	configFile, err := img.ConfigFile()
	if err != nil {
		return "", fmt.Errorf("can't get image config: %w", err)
	}

	versionLabel, ok := configFile.Config.Labels["org.opencontainers.image.version"]
	if !ok {
		klog.Warningf("no org.opencontainers.image.version label found for image: %s", imageReference)
		return "", fmt.Errorf("no version label found")
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("can't get image digest: %w", err)
	}
	return fmt.Sprintf("%s@%s", versionLabel, digest), nil
}

func computeSemVersion(fullVersion string) (string, error) {
	reSemVersion := regexp.MustCompile(`\d+\.\d+\.\d+`)
	semVersion := reSemVersion.FindString(fullVersion)

	if semVersion == "" {
		return "", fmt.Errorf("could not extract semantic version from: %s", fullVersion)
	}

	reSuffix := regexp.MustCompile(`[~-]([a-zA-Z]+\d*)`)
	suffixMatch := reSuffix.FindStringSubmatch(fullVersion)
	if len(suffixMatch) > 1 {
		semVersion = fmt.Sprintf("%s-%s", semVersion, suffixMatch[1])
	}
	return semVersion, nil
}
