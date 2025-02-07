package decimal

import (
	"gopkg.in/inf.v0"
	"reflect"
)

func Marshal(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case inf.Dec:
		return EncInfDec(v)
	case *inf.Dec:
		return EncInfDecR(v)
	case string:
		return EncString(v)
	case *string:
		return EncStringR(v)
	default:
		// Custom types (type MyString string) can be serialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.TypeOf(value)
		if rv.Kind() != reflect.Ptr {
			return EncReflect(reflect.ValueOf(v))
		}
		return EncReflectR(reflect.ValueOf(v))
	}
}
