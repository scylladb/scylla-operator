package timestamp

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
	case *time.Time:
		return DecTime(data, v)
	case **time.Time:
		return DecTimeR(data, v)
	default:

		// Custom types (type MyTime int64) can be deserialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.ValueOf(value)
		rt := rv.Type()
		if rt.Kind() != reflect.Ptr {
			return fmt.Errorf("failed to unmarshal timestamp: unsupported value type (%T)(%[1]v), supported types: ~int64, time.Time", value)
		}
		if rt.Elem().Kind() != reflect.Ptr {
			return DecReflect(data, rv)
		}
		return DecReflectR(data, rv)
	}
}
