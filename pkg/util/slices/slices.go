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
