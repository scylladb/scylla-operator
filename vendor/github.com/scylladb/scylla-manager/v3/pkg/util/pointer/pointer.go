// Copyright (C) 2017 ScyllaDB

package pointer

import "time"

// Int32Ptr returns a pointer to an int32.
func Int32Ptr(i int32) *int32 {
	return &i
}

// Int32PtrDerefOr dereference the int32 ptr and returns it if not nil,
// else returns def.
func Int32PtrDerefOr(ptr *int32, def int32) int32 {
	if ptr != nil {
		return *ptr
	}
	return def
}

// Int64Ptr returns a pointer to an int64.
func Int64Ptr(i int64) *int64 {
	return &i
}

// Int64PtrDerefOr dereference the int64 ptr and returns it if not nil,
// else returns def.
func Int64PtrDerefOr(ptr *int64, def int64) int64 {
	if ptr != nil {
		return *ptr
	}
	return def
}

// BoolPtr returns a pointer to a bool.
func BoolPtr(b bool) *bool {
	return &b
}

// BoolPtrDerefOr dereference the bool ptr and returns it if not nil,
// else returns def.
func BoolPtrDerefOr(ptr *bool, def bool) bool {
	if ptr != nil {
		return *ptr
	}
	return def
}

// StringPtr returns a pointer to the passed string.
func StringPtr(s string) *string {
	return &s
}

// StringPtrDerefOr dereference the string ptr and returns it if not nil,
// else returns def.
func StringPtrDerefOr(ptr *string, def string) string {
	if ptr != nil {
		return *ptr
	}
	return def
}

// Float32Ptr returns a pointer to the passed float32.
func Float32Ptr(i float32) *float32 {
	return &i
}

// Float32PtrDerefOr dereference the float32 ptr and returns it if not nil,
// else returns def.
func Float32PtrDerefOr(ptr *float32, def float32) float32 {
	if ptr != nil {
		return *ptr
	}
	return def
}

// Float64Ptr returns a pointer to the passed float64.
func Float64Ptr(i float64) *float64 {
	return &i
}

// Float64PtrDerefOr dereference the float64 ptr and returns it if not nil,
// else returns def.
func Float64PtrDerefOr(ptr *float64, def float64) float64 {
	if ptr != nil {
		return *ptr
	}
	return def
}

// TimePtr returns a pointer to the passed Time.
func TimePtr(i time.Time) *time.Time {
	return &i
}

// TimePtrDerefOr dereference the time.Time ptr and returns it if not nil,
// else returns def.
func TimePtrDerefOr(ptr *time.Time, def time.Time) time.Time {
	if ptr != nil {
		return *ptr
	}
	return def
}
