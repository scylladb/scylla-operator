package helpers

import (
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func GenerateMergePatch(original runtime.Object, modified runtime.Object) ([]byte, error) {
	originalJSON, err := runtime.Encode(unstructured.UnstructuredJSONScheme, original)
	if err != nil {
		return nil, fmt.Errorf("unable to decode original to JSON: %v", err)
	}

	modifiedJSON, err := runtime.Encode(unstructured.UnstructuredJSONScheme, modified)
	if err != nil {
		return nil, fmt.Errorf("unable to decode modified to JSON: %v", err)
	}

	patchBytes, err := jsonpatch.CreateMergePatch(originalJSON, modifiedJSON)
	if err != nil {
		return nil, fmt.Errorf("unable to create JSON patch: %v", err)
	}

	return patchBytes, nil
}
