package images

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/blobinfocache/none"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// maxImageConfigSize caps how much of an image config blob is read. Image configs are small JSON documents.
const maxImageConfigSize = 4 * 1024 * 1024

// IsImageMultiPlatform checks if the provided image reference is a multi-platform image (a manifest list) that can be
// pulled on each of the required platforms.
// A platform can only be pulled if the list has an entry for it, because that's what container runtimes select on.
// Each entry selected for a required platform must also point to an image whose config is built for that platform.
func IsImageMultiPlatform(ctx context.Context, imageReference string, requiredPlatforms []imgspecv1.Platform) error {
	dockerRef, err := newDockerImageReference(imageReference)
	if err != nil {
		return fmt.Errorf("invalid image reference %q: %w", imageReference, err)
	}

	src, err := dockerRef.NewImageSource(ctx, &types.SystemContext{})
	if err != nil {
		return fmt.Errorf("failed to create image source for %q: %w", imageReference, err)
	}
	defer src.Close()

	manifestBlob, mimeType, err := src.GetManifest(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get manifest for %q: %w", imageReference, err)
	}

	if !slices.Contains(manifest.SupportedListMIMETypes, mimeType) {
		return fmt.Errorf("image %q is not a manifest list, but %q", imageReference, mimeType)
	}

	list, err := manifest.ListFromBlob(manifestBlob, mimeType)
	if err != nil {
		return fmt.Errorf("failed to parse manifest list of %q: %w", imageReference, err)
	}

	var errs []error
	for _, requiredPlatform := range requiredPlatforms {
		var instances []digest.Digest
		for _, instanceDigest := range list.Instances() {
			instance, err := list.Instance(instanceDigest)
			if err != nil {
				return fmt.Errorf("failed to get instance %q of %q: %w", instanceDigest, imageReference, err)
			}

			if instance.ReadOnly.Platform != nil && platformsMatch(*instance.ReadOnly.Platform, requiredPlatform) {
				instances = append(instances, instanceDigest)
			}
		}

		if len(instances) == 0 {
			errs = append(errs, fmt.Errorf("image %q has no manifest list entry for platform %q", imageReference, formatPlatform(requiredPlatform)))
			continue
		}

		for _, instanceDigest := range instances {
			config, err := getInstanceConfig(ctx, src, instanceDigest)
			if err != nil {
				return fmt.Errorf("failed to get config of instance %q of %q: %w", instanceDigest, imageReference, err)
			}

			if config.OS != requiredPlatform.OS || config.Architecture != requiredPlatform.Architecture {
				errs = append(errs, fmt.Errorf("image %q lists instance %q for platform %q, but its config is for platform %q", imageReference, instanceDigest, formatPlatform(requiredPlatform), formatPlatform(config.Platform)))
			}
		}
	}

	return errors.Join(errs...)
}

// getInstanceConfig fetches the config of the image with the given digest.
func getInstanceConfig(ctx context.Context, src types.ImageSource, instanceDigest digest.Digest) (*imgspecv1.Image, error) {
	manifestBlob, mimeType, err := src.GetManifest(ctx, &instanceDigest)
	if err != nil {
		return nil, fmt.Errorf("can't get manifest: %w", err)
	}

	m, err := manifest.FromBlob(manifestBlob, mimeType)
	if err != nil {
		return nil, fmt.Errorf("can't parse manifest: %w", err)
	}

	rc, _, err := src.GetBlob(ctx, m.ConfigInfo(), none.NoCache)
	if err != nil {
		return nil, fmt.Errorf("can't get config blob: %w", err)
	}
	defer rc.Close()

	configBlob, err := io.ReadAll(io.LimitReader(rc, maxImageConfigSize))
	if err != nil {
		return nil, fmt.Errorf("can't read config blob: %w", err)
	}

	config := &imgspecv1.Image{}
	err = json.Unmarshal(configBlob, config)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal config blob: %w", err)
	}

	return config, nil
}

// platformsMatch reports whether the platforms are the same, treating the default variant of an architecture as equal to
// an empty one.
func platformsMatch(a, b imgspecv1.Platform) bool {
	return a.OS == b.OS &&
		a.Architecture == b.Architecture &&
		normalizeVariant(a.Architecture, a.Variant) == normalizeVariant(b.Architecture, b.Variant)
}

func normalizeVariant(architecture, variant string) string {
	if architecture == "arm64" && variant == "v8" {
		return ""
	}

	return variant
}

func formatPlatform(p imgspecv1.Platform) string {
	s := fmt.Sprintf("%s/%s", p.OS, p.Architecture)
	if len(p.Variant) != 0 {
		s += "/" + p.Variant
	}

	return s
}

// newDockerImageReference creates a new Docker-compatible image reference from the provided image string.
func newDockerImageReference(image string) (types.ImageReference, error) {
	r, err := reference.Parse(image)
	if err != nil {
		return nil, fmt.Errorf("invalid image reference %q: %w", image, err)
	}

	named, ok := r.(reference.Named)
	if !ok {
		return nil, fmt.Errorf("image reference %q is not a named reference", image)
	}

	named, err = getNamedOrCanonicalFromTaggedDigestedNamed(named)
	if err != nil {
		return nil, fmt.Errorf("normalizing image reference %q: %w", image, err)
	}

	ref, err := docker.Transport.ParseReference(fmt.Sprintf("//%s", named))
	if err != nil {
		return nil, fmt.Errorf("invalid image reference %q: %w", image, err)
	}

	return ref, nil
}

// getNamedOrCanonicalFromTaggedDigestedNamed normalizes a named reference so it only contains the name and (optional) either a tag or a digest.
// If the named reference is tagged and digested, it will return a new named reference with the tag removed and the digest set (canonical form).
// If the named reference is not tagged or digested, it will return the original named reference (which may be a canonical form already).
func getNamedOrCanonicalFromTaggedDigestedNamed(named reference.Named) (reference.Named, error) {
	_, isTagged := named.(reference.NamedTagged)
	if !isTagged {
		return named, nil
	}
	digested, isDigested := named.(reference.Digested)
	if !isDigested {
		return named, nil
	}

	newNamed := reference.TrimNamed(named)
	newNamed, err := reference.WithDigest(newNamed, digested.Digest())
	if err != nil {
		return named, err
	}
	return newNamed, nil
}
