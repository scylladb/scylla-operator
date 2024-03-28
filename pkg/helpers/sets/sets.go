//  Copyright (C) 2024 ScyllaDB

package sets

type Set[T comparable] map[T]struct{}

func New[T comparable](xs ...T) Set[T] {
	s := make(Set[T], len(xs))
	for _, x := range xs {
		s.Insert(x)
	}
	return s
}

func (s Set[T]) Insert(x T) Set[T] {
	s[x] = struct{}{}
	return s
}

func (s Set[T]) Has(x T) bool {
	_, ok := s[x]
	return ok
}
