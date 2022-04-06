// Copyright (C) 2021 ScyllaDB

package helpers

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

func CreateTwoWayMergePatch[T runtime.Object](original, modified T) ([]byte, error) {
	oldJson, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("can't marshal old object: %w", err)
	}

	newJson, err := json.Marshal(modified)
	if err != nil {
		return nil, fmt.Errorf("can't marshal new object: %w", err)
	}

	return strategicpatch.CreateTwoWayMergePatch(oldJson, newJson, modified)
}
