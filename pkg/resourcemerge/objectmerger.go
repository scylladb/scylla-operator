package resourcemerge

import (
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func isRemovalKey(k string) bool {
	return strings.HasSuffix(k, "-")
}

func toRemovalKey(k string) string {
	return k + "-"
}

func cleanRemovalKeys(m map[string]string) map[string]string {
	for k := range m {
		if isRemovalKey(k) {
			delete(m, k)
		}
	}

	return m
}

func SanitizeObject(obj metav1.Object) {
	cleanRemovalKeys(obj.GetAnnotations())
	cleanRemovalKeys(obj.GetLabels())
}

// MarkMissingKeysForRemoval adds a removal key to required for every key of existing that required doesn't have,
// so that MergeMapInPlaceWithoutRemovalKeys doesn't carry the key over from the existing object. Keys listed in keep
// are left alone.
func MarkMissingKeysForRemoval(required map[string]string, existing map[string]string, keep ...string) {
	for existingKey := range existing {
		if isRemovalKey(existingKey) {
			continue
		}

		if slices.Contains(keep, existingKey) {
			continue
		}

		_, found := required[existingKey]
		if found {
			continue
		}

		required[toRemovalKey(existingKey)] = ""
	}
}

// MergeMapInPlaceWithoutRemovalKeys merges keys from existing into the required object.
func MergeMapInPlaceWithoutRemovalKeys(required map[string]string, existing map[string]string) {
	for existingKey, existingValue := range existing {
		// Don't copy removed keys.
		_, isRemoved := required[toRemovalKey(existingKey)]
		if isRemoved {
			continue
		}

		// Copy only keys not present in the required object.
		_, found := required[existingKey]
		if !found {
			required[existingKey] = existingValue
		}
	}

	cleanRemovalKeys(required)
}

// MergeMetadataInPlace merges metadata from existing into the required.
func MergeMetadataInPlace(required metav1.Object, existing metav1.Object) {
	MergeMapInPlaceWithoutRemovalKeys(required.GetAnnotations(), existing.GetAnnotations())
	MergeMapInPlaceWithoutRemovalKeys(required.GetLabels(), existing.GetLabels())
}
