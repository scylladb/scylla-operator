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
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/blang/semver"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"k8s.io/klog/v2"
)

var (
	cacheScyllaDBImageVersionAndDigest     = make(map[string]string)
	cacheLockScyllaDBImageVersionAndDigest sync.RWMutex
)

var (
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

func GetImageVersionAndDigest(imageType, version string) (string, error) {
	imageReference := fmt.Sprintf("scylladb/%s", imageType)

	if strings.Contains(version, ":") && !strings.Contains(version, "@") {
		imageReference = fmt.Sprintf("%s@%s", imageReference, version)
	} else {
		imageReference = fmt.Sprintf("%s:%s", imageReference, version)
	}

	cacheLockScyllaDBImageVersionAndDigest.RLock()
	if cached, ok := cacheScyllaDBImageVersionAndDigest[imageReference]; ok {
		cacheLockScyllaDBImageVersionAndDigest.RUnlock()
		return cached, nil
	}
	cacheLockScyllaDBImageVersionAndDigest.RUnlock()

	ctx := context.Background()
	transport := docker.Transport

	ref, err := transport.ParseReference(fmt.Sprintf("//%s", imageReference))
	if err != nil {
		return "", fmt.Errorf("can't parse image reference: %w", err)
	}

	sysCtx := &types.SystemContext{}

	src, err := ref.NewImageSource(ctx, sysCtx)
	if err != nil {
		return "", fmt.Errorf("can't get new image source: %w", err)
	}
	defer func() {
		if closeErr := src.Close(); closeErr != nil {
			klog.ErrorS(closeErr, "failed to close image source")
		}
	}()

	img, err := image.FromUnparsedImage(ctx, sysCtx, image.UnparsedInstance(src, nil))
	if err != nil {
		return "", fmt.Errorf("can't read unparsed image: %w", err)
	}

	inspect, err := img.Inspect(ctx)
	if err != nil {
		return "", fmt.Errorf("can't inspect image: %w", err)
	}

	versionLabel, ok := inspect.Labels["org.opencontainers.image.version"]
	if !ok {
		klog.Warningf("no org.opencontainers.image.version label found for image: %s", imageReference)
		return version, nil
	}

	manifestBytes, _, err := img.Manifest(ctx)
	if err != nil {
		return "", fmt.Errorf("can't get image manifest: %w", err)
	}

	digest, err := manifest.Digest(manifestBytes)
	if err != nil {
		return "", fmt.Errorf("can't compute digest: %w", err)
	}

	resolvedVersion := fmt.Sprintf("%s@%s", versionLabel, digest)
	klog.V(4).InfoS("Resolved version", "resolvedVersion", resolvedVersion)
	re := regexp.MustCompile(`\d+\.\d+\.\d+`)
	semverValue := re.FindString(resolvedVersion)

	cacheLockScyllaDBImageVersionAndDigest.Lock()
	cacheScyllaDBImageVersionAndDigest[imageReference] = semverValue
	cacheLockScyllaDBImageVersionAndDigest.Unlock()

	return semverValue, nil
}
