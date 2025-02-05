package double

import (
	"fmt"
	"reflect"
)

func Unmarshal(data []byte, value interface{}) error {
	switch v := value.(type) {
	case nil:
		return nil
	case *float64:
		return DecFloat64(data, v)
	case **float64:
		return DecFloat64R(data, v)
	default:
		// Custom types (type MyFloat float64) can be deserialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.ValueOf(value)
		rt := rv.Type()
		if rt.Kind() != reflect.Ptr {
			return fmt.Errorf("failed to unmarshal double: unsupported value type (%T)(%[1]v)", v)
		}
		if rt.Elem().Kind() != reflect.Ptr {
			return DecReflect(data, rv)
		}
		return DecReflectR(data, rv)
	}
}
