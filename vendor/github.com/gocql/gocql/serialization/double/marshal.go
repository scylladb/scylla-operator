package double

import (
	"reflect"
)

func Marshal(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case float64:
		return EncFloat64(v)
	case *float64:
		return EncFloat64R(v)
	default:
		// Custom types (type MyFloat float64) can be serialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.TypeOf(value)
		if rv.Kind() != reflect.Ptr {
			return EncReflect(reflect.ValueOf(v))
		}
		return EncReflectR(reflect.ValueOf(v))
	}
}
