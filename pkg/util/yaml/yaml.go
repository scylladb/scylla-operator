// Copyright (C) 2017 ScyllaDB

package yaml

import (
	"bufio"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryutilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// ToUnstructured takes a YAML and converts it to a list of Unstructured objects
func ToUnstructured(rawyaml io.Reader) ([]unstructured.Unstructured, error) {
	var objs []unstructured.Unstructured

	reader := apimachineryutilyaml.NewYAMLReader(bufio.NewReader(rawyaml))
	count := 1
	for {
		// Read one YAML document at a time, until io.EOF is returned
		b, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read yaml: %w", err)
		}
		if len(b) == 0 {
			break
		}

		var m map[string]interface{}
		if err := yaml.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("failed to unmarshal the %d yaml document: %q: %w", count, string(b), err)
		}

		var u unstructured.Unstructured
		u.SetUnstructuredContent(m)

		// Ignore empty objects.
		// Empty objects are generated if there are weird things in manifest files like e.g. two --- in a row without a yaml doc in the middle
		if u.Object == nil {
			continue
		}

		objs = append(objs, u)
		count++
	}

	return objs, nil
}
