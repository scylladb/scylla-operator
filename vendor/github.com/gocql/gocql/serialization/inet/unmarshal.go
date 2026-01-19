package inet

import (
	"fmt"
	"net"
	"reflect"
)

func Unmarshal(data []byte, value interface{}) error {
	switch v := value.(type) {
	case nil:
		return nil
	case *[]byte:
		return DecBytes(data, v)
	case **[]byte:
		return DecBytesR(data, v)
	case *net.IP:
		return DecNetIP(data, v)
	case **net.IP:
		return DecNetIPr(data, v)
	case *[4]byte:
		return DecArray4(data, v)
	case **[4]byte:
		return DecArray4R(data, v)
	case *[16]byte:
		return DecArray16(data, v)
	case **[16]byte:
		return DecArray16R(data, v)
	case *string:
		return DecString(data, v)
	case **string:
		return DecStringR(data, v)
	default:
		// Custom types (type MyIP []byte) can be deserialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.ValueOf(value)
		rt := rv.Type()
		if rt.Kind() != reflect.Ptr {
			return fmt.Errorf("failed to unmarshal inet: unsupported value type (%T)(%[1]v), supported types: ~[]byte, ~[4]byte, ~[16]byte, ~string, net.IP", v)
		}
		if rt.Elem().Kind() != reflect.Ptr {
			return DecReflect(data, rv)
		}
		return DecReflectR(data, rv)
	}
}
