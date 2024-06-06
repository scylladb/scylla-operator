// Copyright (C) 2021 ScyllaDB

package hash

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"hash"
	"hash/fnv"
)

func HashObjectsShort(objs ...interface{}) (string, error) {
	return hashObjects(fnv.New32a(), objs...)
}

func HashObjects(objs ...interface{}) (string, error) {
	return hashObjects(sha512.New(), objs...)
}

func hashObjects(hasher hash.Hash, objs ...interface{}) (string, error) {
	encoder := json.NewEncoder(hasher)
	for _, obj := range objs {
		if err := encoder.Encode(obj); err != nil {
			return "", err
		}
	}

	return base64.StdEncoding.EncodeToString(hasher.Sum(nil)), nil
}
