// Copyright (C) 2017 ScyllaDB

package store

import (
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/service"
)

type entryHolder struct {
	Entry
	data []byte
}

func (v *entryHolder) MarshalBinary() (data []byte, err error) {
	return v.data, nil
}

func (v *entryHolder) UnmarshalBinary(data []byte) error {
	v.data = data
	return nil
}

// PutWithRollback gets former value of entry and returns a function to restore it.
func PutWithRollback(store Store, e Entry) (func(), error) {
	old := &entryHolder{
		Entry: e,
	}

	if err := store.Get(old); err != nil && !errors.Is(err, service.ErrNotFound) {
		return nil, errors.Wrap(err, "get")
	}
	if err := store.Put(e); err != nil {
		return nil, errors.Wrap(err, "put")
	}
	return func() {
		if old.data == nil {
			store.Delete(old) // nolint: errcheck
		} else {
			store.Put(old) // nolint: errcheck
		}
	}, nil
}
