package cqltime

import (
	"fmt"
	"reflect"
	"time"
)

func Unmarshal(data []byte, value interface{}) error {
	switch v := value.(type) {
	case nil:
		return nil

	case *int64:
		return DecInt64(data, v)
	case **int64:
		return DecInt64R(data, v)
	case *time.Duration:
		return DecDuration(data, v)
	case **time.Duration:
		return DecDurationR(data, v)
	default:

		// Custom types (type MyTime int64) can be deserialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.ValueOf(value)
		rt := rv.Type()
		if rt.Kind() != reflect.Ptr {
			return fmt.Errorf("failed to unmarshal time: unsupported value type (%T)(%[1]v), supported types: ~int64, time.Duration", value)
		}
		if rt.Elem().Kind() != reflect.Ptr {
			return DecReflect(data, rv)
		}
		return DecReflectR(data, rv)
	}
}
