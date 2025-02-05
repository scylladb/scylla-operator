package varint

import (
	"fmt"
	"math/big"
	"reflect"
)

func Unmarshal(data []byte, value interface{}) error {
	switch v := value.(type) {
	case nil:
		return nil

	case *int8:
		return DecInt8(data, v)
	case *int16:
		return DecInt16(data, v)
	case *int32:
		return DecInt32(data, v)
	case *int64:
		return DecInt64(data, v)
	case *int:
		return DecInt(data, v)

	case *uint8:
		return DecUint8(data, v)
	case *uint16:
		return DecUint16(data, v)
	case *uint32:
		return DecUint32(data, v)
	case *uint64:
		return DecUint64(data, v)
	case *uint:
		return DecUint(data, v)

	case *big.Int:
		return DecBigInt(data, v)
	case *string:
		return DecString(data, v)

	case **int8:
		return DecInt8R(data, v)
	case **int16:
		return DecInt16R(data, v)
	case **int32:
		return DecInt32R(data, v)
	case **int64:
		return DecInt64R(data, v)
	case **int:
		return DecIntR(data, v)

	case **uint8:
		return DecUint8R(data, v)
	case **uint16:
		return DecUint16R(data, v)
	case **uint32:
		return DecUint32R(data, v)
	case **uint64:
		return DecUint64R(data, v)
	case **uint:
		return DecUintR(data, v)

	case **big.Int:
		return DecBigIntR(data, v)
	case **string:
		return DecStringR(data, v)
	default:

		// Custom types (type MyInt int) can be deserialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.ValueOf(value)
		rt := rv.Type()
		if rt.Kind() != reflect.Ptr {
			return fmt.Errorf("failed to unmarshal varint: unsupported value type (%T)(%#[1]v)", value)
		}
		if rt.Elem().Kind() != reflect.Ptr {
			return DecReflect(data, rv)
		}
		return DecReflectR(data, rv)
	}
}
