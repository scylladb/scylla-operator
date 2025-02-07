package date

import (
	"reflect"
	"time"
)

func Marshal(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case int32:
		return EncInt32(v)
	case int64:
		return EncInt64(v)
	case uint32:
		return EncUint32(v)
	case string:
		return EncString(v)
	case time.Time:
		return EncTime(v)

	case *int32:
		return EncInt32R(v)
	case *int64:
		return EncInt64R(v)
	case *uint32:
		return EncUint32R(v)
	case *string:
		return EncStringR(v)
	case *time.Time:
		return EncTimeR(v)
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
