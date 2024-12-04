// Copyright (c) 2024 ScyllaDB.

package lazy

import "sync"

type Value[T any] struct {
	init    sync.Once
	newFunc func() T
	value   T
}

// New returns a Value which is initialized upon first call to Get using newFunc.
func New[T any](newFunc func() T) *Value[T] {
	return &Value[T]{
		init:    sync.Once{},
		newFunc: newFunc,
	}
}

func (li *Value[T]) Get() T {
	li.init.Do(func() {
		li.value = li.newFunc()
	})
	return li.value
}
