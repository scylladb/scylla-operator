package uuid

import (
	"fmt"
	"reflect"
)

func Unmarshal(data []byte, value interface{}) error {
	switch v := value.(type) {
	case nil:
		return nil
	case *[16]byte:
		return DecArray(data, v)
	case **[16]byte:
		return DecArrayR(data, v)
	case *[]byte:
		return DecSlice(data, v)
	case **[]byte:
		return DecSliceR(data, v)
	case *string:
		return DecString(data, v)
	case **string:
		return DecStringR(data, v)
	default:
		// Custom types (type MyFloat float32) can be deserialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.ValueOf(value)
		if rv.Kind() != reflect.Ptr {
			return fmt.Errorf("failed to unmarshal uuid: unsupported value type (%T)(%[1]v)", v)
		}
		if rv.Type().Elem().Kind() != reflect.Ptr {
			return DecReflect(data, rv)
		}
		return DecReflectR(data, rv)
	}
}
