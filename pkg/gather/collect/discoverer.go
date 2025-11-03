package collect

import (
	"fmt"
	"strings"

	"github.com/scylladb/scylla-operator/pkg/helpers"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
)

// DefaultExcludedSensitiveResources returns resources that should never be collected by default, unless explicitly overridden by the user.
func DefaultExcludedSensitiveResources() []schema.GroupKind {
	return []schema.GroupKind{
		// Built-in Secrets.
		{
			Group: "",
			Kind:  "Secret",
		},
		// Bitnami SealedSecrets.
		{
			Group: "bitnami.com",
			Kind:  "SealedSecret",
		},
	}
}

// ResourceInfo holds information about a Kubernetes resource.
type ResourceInfo struct {
	Scope    meta.RESTScope
	Resource schema.GroupVersionResource
}

type ServerPreferredResourcesGetter interface {
	ServerPreferredResources() ([]*metav1.APIResourceList, error)
}

// ResourceDiscoverer discovers resources available in the cluster.
type ResourceDiscoverer struct {
	includeSensitiveResources      bool
	serverPreferredResourcesGetter ServerPreferredResourcesGetter
}

func NewResourceDiscoverer(includeSensitiveResources bool, serverPreferredResourcesGetter ServerPreferredResourcesGetter) *ResourceDiscoverer {
	return &ResourceDiscoverer{
		includeSensitiveResources:      includeSensitiveResources,
		serverPreferredResourcesGetter: serverPreferredResourcesGetter,
	}
}

// DiscoverResources returns ResourceInfo for all resources available in the cluster with a server preferred version.
// It excludes resources specified in userProvidedResourcesToExclude and defaultExcludedSensitiveResources.
func (d *ResourceDiscoverer) DiscoverResources(userProvidedResourcesToExclude ...schema.GroupKind) ([]*ResourceInfo, error) {
	rls, err := d.serverPreferredResourcesGetter.ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("can't discover resources: %w", err)
	}

	// Apply all filters.
	for _, f := range d.prepareFilters(rls, userProvidedResourcesToExclude) {
		rls = discovery.FilteredBy(f, rls)
	}

	// There should be at least one resource per group, likely more.
	resourceInfos := make([]*ResourceInfo, 0, len(rls))
	for _, rl := range rls {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			return nil, err
		}

		for _, r := range rl.APIResources {
			var scope meta.RESTScope
			if r.Namespaced {
				scope = meta.RESTScopeNamespace
			} else {
				scope = meta.RESTScopeRoot
			}
			resourceInfos = append(resourceInfos, &ResourceInfo{
				Scope:    scope,
				Resource: gv.WithResource(r.Name),
			})
		}
	}

	// To avoid collecting duplicate data for resources that share storage across API groups,
	// we replace older isometric resources with their newer counterparts.
	resourceInfos, err = replaceIsometricResourceInfosIfPresent(resourceInfos)
	if err != nil {
		return nil, fmt.Errorf("can't replace isometric resource infos: %w", err)
	}

	return resourceInfos, nil
}

func (d *ResourceDiscoverer) prepareFilters(existingResources []*metav1.APIResourceList, userProvidedResourcesToExclude []schema.GroupKind) []discovery.ResourcePredicateFunc {
	var filters []discovery.ResourcePredicateFunc

	// Always require 'list' verb.
	filters = append(filters, discovery.SupportsAllVerbs{Verbs: []string{"list"}}.Match)

	// Exclusion filters.
	filters = append(filters, d.prepareExclusionFilters(existingResources, userProvidedResourcesToExclude)...)

	return filters
}

func (d *ResourceDiscoverer) prepareExclusionFilters(existingResources []*metav1.APIResourceList, userProvidedResourcesToExclude []schema.GroupKind) []discovery.ResourcePredicateFunc {
	var resourcesToExclude []schema.GroupKind

	// User-provided exclusions.
	existingUserProvidedResourcesToExclude := getExistingResourcesOnly(existingResources, userProvidedResourcesToExclude)
	resourcesToExclude = append(resourcesToExclude, existingUserProvidedResourcesToExclude...)

	// Default sensitive resource exclusions.
	if !d.includeSensitiveResources {
		resourcesToExclude = append(resourcesToExclude, DefaultExcludedSensitiveResources()...)
	}

	var filters []discovery.ResourcePredicateFunc
	for _, gk := range resourcesToExclude {
		filters = append(filters, ignoreResourceByGroupKind(gk))
		klog.InfoS("Excluding resource from collection", "group", gk.Group, "kind", gk.Kind)
	}

	return filters
}

func replaceIsometricResourceInfosIfPresent(resourceInfos []*ResourceInfo) ([]*ResourceInfo, error) {
	// Replacements are order dependent and should start from the oldest API.
	replacements := []struct {
		old schema.GroupVersionResource
		new schema.GroupVersionResource
	}{
		{
			old: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "events",
			},
			new: schema.GroupVersionResource{
				Group:    "events.k8s.io",
				Version:  "v1",
				Resource: "events",
			},
		},
	}

	resourceInfosMap := make(map[schema.GroupVersionResource]*ResourceInfo, len(resourceInfos))
	for _, m := range resourceInfos {
		resourceInfosMap[m.Resource] = m
	}

	for _, replacement := range replacements {
		_, found := resourceInfosMap[replacement.new]
		if found {
			delete(resourceInfosMap, replacement.old)
		}
	}

	return helpers.GetMapValues(resourceInfosMap), nil
}

// ignoreResourceByGroupKind returns a ResourcePredicateFunc that ignores resources matching the given GroupKind.
// Kind is compared in a case-insensitive manner.
func ignoreResourceByGroupKind(gk schema.GroupKind) discovery.ResourcePredicateFunc {
	return func(gvString string, r *metav1.APIResource) bool {
		// We have to parse the groupVersion string to get the group.
		// Note: not using r.Group because it may be empty for Kubernetes built-in resources (https://github.com/kubernetes/kubernetes/blob/c30b578ee59ad0b1505009897a2a3d7766a6073e/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/types.go#L1182).
		group := getGroupFromGroupVersionString(gvString)
		return !groupKindEquals(schema.GroupKind{Group: group, Kind: r.Kind}, gk)
	}
}

// getExistingResourcesOnly filters the provided resources to exclude, returning only those that exist in the discovered resources.
func getExistingResourcesOnly(discoveredResources []*metav1.APIResourceList, resourcesToExclude []schema.GroupKind) []schema.GroupKind {
	var gks []schema.GroupKind

	for _, gk := range resourcesToExclude {
		// Ensure the resource we want to exclude actually exists - otherwise, log a warning.
		found := findResource(discoveredResources, gk)
		if !found {
			klog.InfoS("Warning: Resource to exclude from collection not found in the cluster, ensure you provided correct group and kind", "group", gk.Group, "kind", gk.Kind)
			continue
		}

		gks = append(gks, gk)
	}

	return gks
}

func findResource(discoveredResources []*metav1.APIResourceList, gk schema.GroupKind) bool {
	for _, rl := range discoveredResources {
		for _, r := range rl.APIResources {
			group := getGroupFromGroupVersionString(rl.GroupVersion)
			if groupKindEquals(schema.GroupKind{Group: group, Kind: r.Kind}, gk) {
				return true
			}
		}
	}
	return false
}

// groupKindEquals compares two GroupKind objects for equality, with case-insensitive Kind comparison.
func groupKindEquals(a, b schema.GroupKind) bool {
	return a.Group == b.Group && strings.EqualFold(a.Kind, b.Kind)
}

func getGroupFromGroupVersionString(gvString string) string {
	gv, err := schema.ParseGroupVersion(gvString)
	if err != nil {
		// This should never happen as the groupVersion string comes from the server.
		klog.ErrorS(err, "Failed to parse groupVersion string that came from server, this should never happen", "groupVersion", gvString)
		return ""
	}

	return gv.Group
}
