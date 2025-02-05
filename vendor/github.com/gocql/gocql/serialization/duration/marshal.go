package duration

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
	case time.Duration:
		return EncDur(v)
	case string:
		return EncString(v)
	case Duration:
		return EncDuration(v)

	case *int64:
		return EncInt64R(v)
	case *time.Duration:
		return EncDurR(v)
	case *string:
		return EncStringR(v)
	case *Duration:
		return EncDurationR(v)
	default:
		// Custom types (type MyDate uint32) can be serialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.TypeOf(value)
		if rv.Kind() != reflect.Ptr {
			return EncReflect(reflect.ValueOf(v))
		}
		return EncReflectR(reflect.ValueOf(v))
	}
}
