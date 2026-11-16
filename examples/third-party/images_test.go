// Copyright (C) 2026 ScyllaDB

package thirdparty

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	configassets "github.com/scylladb/scylla-operator/assets/config"
	"github.com/scylladb/scylla-operator/pkg/util/images"
	"github.com/scylladb/scylla-operator/pkg/util/yaml"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachineryutilsets "k8s.io/apimachinery/pkg/util/sets"
)

// podSpecPaths maps the kinds of workloads to the path of their pod spec.
var podSpecPaths = map[schema.GroupKind][]string{
	{Group: "", Kind: "Pod"}:                   {"spec"},
	{Group: "apps", Kind: "Deployment"}:        {"spec", "template", "spec"},
	{Group: "apps", Kind: "DaemonSet"}:         {"spec", "template", "spec"},
	{Group: "apps", Kind: "StatefulSet"}:       {"spec", "template", "spec"},
	{Group: "apps", Kind: "ReplicaSet"}:        {"spec", "template", "spec"},
	{Group: "batch", Kind: "Job"}:              {"spec", "template", "spec"},
	{Group: "batch", Kind: "CronJob"}:          {"spec", "jobTemplate", "spec", "template", "spec"},
	{Group: "", Kind: "ReplicationController"}: {"spec", "template", "spec"},
}

// imageArgPrefixes are the prefixes of container args that hold images the container deploys.
// Each one has to be found in the manifests, so the test can't silently stop covering an image when an arg is renamed.
var imageArgPrefixes = []string{
	// prometheus-operator injects the config reloader as a sidecar into the Prometheus pods.
	"--prometheus-config-reloader=",
}

// TestThirdPartyImagesAreMultiPlatform verifies that every image deployed by the third-party manifests can be pulled
// on all supported platforms.
func TestThirdPartyImagesAreMultiPlatform(t *testing.T) {
	t.Parallel()

	manifestImages := apimachineryutilsets.New[string]()
	foundImageArgPrefixes := apimachineryutilsets.New[string]()

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("can't open %q: %v", path, err)
		}
		defer f.Close()

		objs, err := yaml.ToUnstructured(f)
		if err != nil {
			t.Fatalf("can't decode %q: %v", path, err)
		}

		for _, obj := range objs {
			podSpec, ok := getPodSpec(t, obj)
			if !ok {
				continue
			}

			for _, c := range slices.Concat(podSpec.InitContainers, podSpec.Containers) {
				manifestImages.Insert(c.Image)

				for _, arg := range c.Args {
					for _, prefix := range imageArgPrefixes {
						if image, ok := strings.CutPrefix(arg, prefix); ok {
							manifestImages.Insert(image)
							foundImageArgPrefixes.Insert(prefix)
						}
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("can't walk the third-party manifests: %v", err)
	}

	if missing := apimachineryutilsets.New(imageArgPrefixes...).Difference(foundImageArgPrefixes); missing.Len() != 0 {
		t.Errorf("image args %q weren't found in any third-party manifest", apimachineryutilsets.List(missing))
	}

	if manifestImages.Len() == 0 {
		t.Fatal("no images found in the third-party manifests")
	}

	for _, image := range apimachineryutilsets.List(manifestImages) {
		t.Run(image, func(t *testing.T) {
			t.Parallel()

			err := images.IsImageMultiPlatform(t.Context(), image, configassets.SupportedPlatforms)
			if err != nil {
				t.Error(err)
			}
		})
	}
}

func getPodSpec(t *testing.T, obj unstructured.Unstructured) (*corev1.PodSpec, bool) {
	t.Helper()

	path, ok := podSpecPaths[obj.GroupVersionKind().GroupKind()]
	if !ok {
		return nil, false
	}

	podSpecMap, found, err := unstructured.NestedMap(obj.Object, path...)
	if err != nil {
		t.Fatalf("can't get pod spec of %s %q: %v", obj.GetKind(), obj.GetName(), err)
	}
	if !found {
		t.Fatalf("%s %q has no pod spec at %q", obj.GetKind(), obj.GetName(), strings.Join(path, "."))
	}

	podSpec := &corev1.PodSpec{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(podSpecMap, podSpec)
	if err != nil {
		t.Fatalf("can't convert pod spec of %s %q: %v", obj.GetKind(), obj.GetName(), err)
	}

	return podSpec, true
}
