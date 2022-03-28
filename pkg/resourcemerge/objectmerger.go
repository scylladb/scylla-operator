package resourcemerge

import (
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

// MergeMapInPlaceWithoutRemovalKeys merges keys from existing into the required object.
func MergeMapInPlaceWithoutRemovalKeys(required *map[string]string, existing map[string]string) {
	for existingKey, existingValue := range existing {
		// Don't copy removed keys.
		_, isRemoved := (*required)[toRemovalKey(existingKey)]
		if isRemoved {
			continue
		}

		// Copy only keys not present in the required object.
		_, found := (*required)[existingKey]
		if !found {
			(*required)[existingKey] = existingValue
		}
	}

	cleanRemovalKeys(*required)
}

// MergeMetadataInPlace merges metadata from existing into the required.
func MergeMetadataInPlace(required *metav1.ObjectMeta, existing metav1.ObjectMeta) {
	MergeMapInPlaceWithoutRemovalKeys(&required.Annotations, existing.Annotations)
	MergeMapInPlaceWithoutRemovalKeys(&required.Labels, existing.Labels)
}
