package counter

import (
	"math/big"
	"reflect"
)

func Marshal(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case int8:
		return EncInt8(v)
	case int16:
		return EncInt16(v)
	case int32:
		return EncInt32(v)
	case int64:
		return EncInt64(v)
	case int:
		return EncInt(v)

	case uint8:
		return EncUint8(v)
	case uint16:
		return EncUint16(v)
	case uint32:
		return EncUint32(v)
	case uint64:
		return EncUint64(v)
	case uint:
		return EncUint(v)

	case big.Int:
		return EncBigInt(v)
	case string:
		return EncString(v)

	case *int8:
		return EncInt8R(v)
	case *int16:
		return EncInt16R(v)
	case *int32:
		return EncInt32R(v)
	case *int64:
		return EncInt64R(v)
	case *int:
		return EncIntR(v)

	case *uint8:
		return EncUint8R(v)
	case *uint16:
		return EncUint16R(v)
	case *uint32:
		return EncUint32R(v)
	case *uint64:
		return EncUint64R(v)
	case *uint:
		return EncUintR(v)

	case *big.Int:
		return EncBigIntR(v)
	case *string:
		return EncStringR(v)
	default:
		// Custom types (type MyInt int) can be serialized only via `reflect` package.
		// Later, when generic-based serialization is introduced we can do that via generics.
		rv := reflect.TypeOf(value)
		if rv.Kind() != reflect.Ptr {
			return EncReflect(reflect.ValueOf(v))
		}
		return EncReflectR(reflect.ValueOf(v))
	}
}
