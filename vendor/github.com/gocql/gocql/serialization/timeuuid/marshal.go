package timeuuid

import (
	"reflect"
)

func Marshal(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case [16]byte:
		return EncArray(v)
	case *[16]byte:
		return EncArrayR(v)
	case []byte:
		return EncSlice(v)
	case *[]byte:
		return EncSliceR(v)
	case string:
		return EncString(v)
	case *string:
		return EncStringR(v)
	default:
		// Custom types (type MyUUID [16]byte) can be serialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.ValueOf(value)
		if rv.Kind() != reflect.Ptr {
			return EncReflect(rv)
		}
		return EncReflectR(rv)
	}
}
