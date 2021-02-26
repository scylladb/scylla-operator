// Copyright (C) 2021 ScyllaDB

package resourceapply

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"

	"github.com/scylladb/scylla-operator/pkg/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func HashObjects(objs ...interface{}) (string, error) {
	hasher := sha512.New()
	for _, obj := range objs {
		if err := json.NewEncoder(hasher).Encode(obj); err != nil {
			return "", err
		}
	}

	return base64.StdEncoding.EncodeToString(hasher.Sum(nil)), nil
}

func MustHashObjects(objs ...interface{}) string {
	hash, err := HashObjects(objs...)
	if err != nil {
		panic(err)
	}
	return hash
}

func setHash(obj metav1.Object) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}

	// Clear annotation to have consistent hashing for the same objects.
	delete(annotations, naming.ManagedHash)

	hash, err := HashObjects(obj)
	if err != nil {
		return err
	}

	annotations[naming.ManagedHash] = hash
	obj.SetAnnotations(annotations)
	return nil
}
