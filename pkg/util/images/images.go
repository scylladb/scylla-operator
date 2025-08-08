package images

import (
	"context"
	"fmt"
	"slices"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
)

// IsImageMultiPlatform checks if the provided image reference is a multi-platform image (a manifest list).
func IsImageMultiPlatform(ctx context.Context, imageReference string) error {
	dockerRef, err := newDockerImageReference(imageReference)
	if err != nil {
		return fmt.Errorf("invalid image reference %q: %w", imageReference, err)
	}

	src, err := dockerRef.NewImageSource(ctx, &types.SystemContext{})
	if err != nil {
		return fmt.Errorf("failed to create image source for %q: %w", imageReference, err)
	}
	defer src.Close()

	_, mimeType, err := src.GetManifest(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get manifest for %q: %w", imageReference, err)
	}

	multiArchMimeTypes := []string{
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.index.v1+json",
	}
	if !slices.Contains(multiArchMimeTypes, mimeType) {
		return fmt.Errorf("image %q is not a manifest list, but %q", imageReference, mimeType)
	}

	return nil
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
