// Copyright (C) 2021 ScyllaDB

package hash

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
)

func HashObjects(objs ...interface{}) (string, error) {
	hasher := sha512.New()
	encoder := json.NewEncoder(hasher)
	for _, obj := range objs {
		if err := encoder.Encode(obj); err != nil {
			return "", err
		}
	}

	return base64.StdEncoding.EncodeToString(hasher.Sum(nil)), nil
}

func hashObjects(encoding *base64.Encoding, objs ...interface{}) (string, error) {
	hasher := sha512.New()
	encoder := json.NewEncoder(hasher)
	for _, obj := range objs {
		if err := encoder.Encode(obj); err != nil {
			return "", err
		}
	}

	return encoding.EncodeToString(hasher.Sum(nil)), nil
}

func ShortURLHashObjects(objs ...interface{}) (string, error) {
	hash, err := hashObjects(base64.URLEncoding, objs...)
	if err != nil {
		return "", err
	}

	return hash[:12], nil
}
