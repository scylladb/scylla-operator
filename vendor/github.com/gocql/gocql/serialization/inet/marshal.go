package inet

import (
	"net"
	"reflect"
)

func Marshal(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return EncBytes(v)
	case *[]byte:
		return EncBytesR(v)
	case net.IP:
		return EncNetIP(v)
	case *net.IP:
		return EncNetIPr(v)
	case [4]byte:
		return EncArray4(v)
	case *[4]byte:
		return EncArray4R(v)
	case [16]byte:
		return EncArray16(v)
	case *[16]byte:
		return EncArray16R(v)
	case string:
		return EncString(v)
	case *string:
		return EncStringR(v)
	default:
		// Custom types (type MyIP []byte) can be serialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.TypeOf(value)
		if rv.Kind() != reflect.Ptr {
			return EncReflect(reflect.ValueOf(v))
		}
		return EncReflectR(reflect.ValueOf(v))
	}
}
