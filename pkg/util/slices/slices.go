// Copyright (C) 2017 ScyllaDB

package slices

// ContainsString returns whether value is part of array.
func ContainsString(value string, array []string) bool {
	for _, elem := range array {
		if elem == value {
			return true
		}
	}
	return false
}

func RemoveString[T comparable](value T, array []T) []T {
	var filtered []T
	for _, v := range array {
		if v == value {
			continue
		}
		filtered = append(filtered, v)
	}
	return filtered
}
