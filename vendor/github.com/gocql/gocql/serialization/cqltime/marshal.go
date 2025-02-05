package cqltime

import (
	"reflect"
	"time"
)

func Marshal(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case int64:
		return EncInt64(v)
	case *int64:
		return EncInt64R(v)
	case time.Duration:
		return EncDuration(v)
	case *time.Duration:
		return EncDurationR(v)

	default:
		// Custom types (type MyTime int64) can be serialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.TypeOf(value)
		if rv.Kind() != reflect.Ptr {
			return EncReflect(reflect.ValueOf(v))
		}
		return EncReflectR(reflect.ValueOf(v))
	}
}
