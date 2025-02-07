package varchar

import (
	"reflect"
)

func Marshal(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string:
		return EncString(v)
	case *string:
		return EncStringR(v)
	case []byte:
		return EncBytes(v)
	case *[]byte:
		return EncBytesR(v)
	default:
		// Custom types (type MyString string) can be serialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.ValueOf(value)
		if rv.Kind() != reflect.Ptr {
			return EncReflect(rv)
		}
		return EncReflectR(rv)
	}
}
