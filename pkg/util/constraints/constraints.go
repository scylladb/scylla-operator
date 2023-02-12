// Copyright (c) 2023 ScyllaDB.

package constraints

// Unsigned is a constraint that any unsigned integer type satisfies.
type Unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Signed is a constraint that any signed integer type satisfies.
type Signed interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// Integer is a constraint that any integer type satisfies.
type Integer interface {
	Unsigned | Signed
}

// Float is a constraint that any floating type satisfies.
type Float interface {
	~float32 | ~float64
}

// Number is a constraint that any numeric type satisfies.
type Number interface {
	Integer | Float
}
