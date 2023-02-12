// Copyright (c) 2023 ScyllaDB.

package algorithms

import "github.com/scylladb/scylla-operator/pkg/util/constraints"

func Max[T constraints.Number](a T, tail ...T) T {
	max := a
	for _, v := range tail {
		if v > max {
			max = v
		}
	}
	return max
}
