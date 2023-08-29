// Copyright (c) 2023 ScyllaDB

package slices

func ToSlice[T any](objs ...T) []T {
	res := make([]T, 0, len(objs))
	return append(res, objs...)
}

func ConvertToSlice[To, From any](convert func(From) To, objs ...From) []To {
	res := make([]To, 0, len(objs))

	for i := range objs {
		res = append(res, convert(objs[i]))
	}

	return res
}

func ConvertSlice[To, From any](slice []From, convert func(From) To) []To {
	return ConvertToSlice(convert, slice...)
}

func Filter[T any](array []T, filterFunc func(T) bool) []T {
	res := make([]T, 0, len(array))

	for i := range array {
		if filterFunc(array[i]) {
			res = append(res, array[i])
		}
	}

	return res
}

func FilterOut[T any](array []T, filterOutFunc func(T) bool) []T {
	return Filter(array, func(t T) bool {
		return !filterOutFunc(t)
	})
}

func FilterOutNil[T any](array []*T) []*T {
	return FilterOut[*T](array, func(item *T) bool {
		return item == nil
	})
}

func Contains[T any](array []T, cmp func(v T) bool) bool {
	for _, item := range array {
		if cmp(item) {
			return true
		}
	}

	return false
}

func IdentityFunc[T comparable](item T) func(T) bool {
	return func(v T) bool {
		return v == item
	}
}

func ContainsItem[T comparable](slice []T, item T) bool {
	return Contains(slice, IdentityFunc(item))
}

func Find[T any](slice []T, filterFunc func(T) bool) (T, int, bool) {
	for i := range slice {
		if filterFunc(slice[i]) {
			return slice[i], i, true
		}
	}

	return *new(T), 0, false
}

func FindItem[T comparable](slice []T, item T) (T, int, bool) {
	return Find(slice, IdentityFunc(item))
}

func Flatten[T any](xs [][]T) []T {
	var res []T

	for _, x := range xs {
		res = append(res, x...)
	}

	return res
}

func ToString[T ~string](v T) string {
	return string(v)
}
