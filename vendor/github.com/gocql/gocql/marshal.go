/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
/*
 * Content before git sha 34fdeebefcbf183ed7f916f931aa0586fdaa1b40
 * Copyright (c) 2012, The Gocql authors,
 * provided under the BSD-3-Clause License.
 * See the NOTICE file distributed with this work for additional information.
 */

package gocql

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"reflect"
	"unsafe"

	"github.com/gocql/gocql/serialization/ascii"
	"github.com/gocql/gocql/serialization/bigint"
	"github.com/gocql/gocql/serialization/blob"
	"github.com/gocql/gocql/serialization/boolean"
	"github.com/gocql/gocql/serialization/counter"
	"github.com/gocql/gocql/serialization/cqlint"
	"github.com/gocql/gocql/serialization/cqltime"
	"github.com/gocql/gocql/serialization/date"
	"github.com/gocql/gocql/serialization/decimal"
	"github.com/gocql/gocql/serialization/double"
	"github.com/gocql/gocql/serialization/duration"
	"github.com/gocql/gocql/serialization/float"
	"github.com/gocql/gocql/serialization/inet"
	"github.com/gocql/gocql/serialization/smallint"
	"github.com/gocql/gocql/serialization/text"
	"github.com/gocql/gocql/serialization/timestamp"
	"github.com/gocql/gocql/serialization/timeuuid"
	"github.com/gocql/gocql/serialization/tinyint"
	"github.com/gocql/gocql/serialization/uuid"
	"github.com/gocql/gocql/serialization/varchar"
	"github.com/gocql/gocql/serialization/varint"
)

var (
	emptyValue reflect.Value
)

var (
	ErrorUDTUnavailable = errors.New("UDT are not available on protocols less than 3, please update config")
)

// Marshaler is an interface for custom unmarshaler.
// Each value of the 'CQL binary protocol' consist of <value_len> and <value_data>.
// <value_len> can be 'unset'(-2), 'nil'(-1), 'zero'(0) or any value up to 2147483647.
// When <value_len> is 'unset', 'nil' or 'zero', <value_data> is not present.
// 'unset' is applicable only to columns, with some exceptions.
// As you can see from API MarshalCQL only returns <value_data>, but there is a way for it to control <value_len>:
//  1. If MarshalCQL returns (gocql.UnsetValue, nil), gocql writes 'unset' to <value_len>
//  2. If MarshalCQL returns ([]byte(nil), nil), gocql writes 'nil' to <value_len>
//  3. If MarshalCQL returns ([]byte{}, nil), gocql writes 'zero' to <value_len>
//
// Some CQL databases have proprietary value coding features, which you may want to consider.
// CQL binary protocol info:https://github.com/apache/cassandra/tree/trunk/doc
type Marshaler interface {
	MarshalCQL(info TypeInfo) ([]byte, error)
}

type DirectMarshal []byte

func (m DirectMarshal) MarshalCQL(_ TypeInfo) ([]byte, error) {
	return m, nil
}

// Unmarshaler is an interface for custom unmarshaler.
// Each value of the 'CQL binary protocol' consist of <value_len> and <value_data>.
// <value_len> can be 'unset'(-2), 'nil'(-1), 'zero'(0) or any value up to 2147483647.
// When <value_len> is 'unset', 'nil' or 'zero', <value_data> is not present.
// As you can see from an API UnmarshalCQL receives only 'info TypeInfo' and
// 'data []byte', but gocql has the following way to signal about <value_len>:
//  1. When <value_len> is 'nil' gocql feeds nil to 'data []byte'
//  2. When <value_len> is 'zero' gocql feeds []byte{} to 'data []byte'
//
// Some CQL databases have proprietary value coding features, which you may want to consider.
// CQL binary protocol info:https://github.com/apache/cassandra/tree/trunk/doc
type Unmarshaler interface {
	UnmarshalCQL(info TypeInfo, data []byte) error
}

type DirectUnmarshal []byte

func (d *DirectUnmarshal) UnmarshalCQL(_ TypeInfo, data []byte) error {
	*d = bytes.Clone(data)
	return nil
}

// Marshal returns the CQL encoding of the value for the Cassandra
// internal type described by the info parameter.
//
// nil is serialized as CQL null.
// If value implements Marshaler, its MarshalCQL method is called to marshal the data.
// If value is a pointer, the pointed-to value is marshaled.
//
// Supported conversions are as follows, other type combinations may be added in the future:
//
//	CQL type                    | Go type (value)    | Note
//	varchar, ascii, blob, text  | string, []byte     |
//	boolean                     | bool               |
//	tinyint, smallint, int      | integer types      |
//	tinyint, smallint, int      | string             | formatted as base 10 number
//	bigint, counter             | integer types      |
//	bigint, counter             | big.Int            | value limited as int64
//	bigint, counter             | string             | formatted as base 10 number
//	float                       | float32            |
//	double                      | float64            |
//	decimal                     | inf.Dec            |
//	time                        | int64              | nanoseconds since start of day
//	time                        | time.Duration      | duration since start of day
//	timestamp                   | int64              | milliseconds since Unix epoch
//	timestamp                   | time.Time          |
//	list, set                   | slice, array       |
//	list, set                   | map[X]struct{}     |
//	map                         | map[X]Y            |
//	uuid, timeuuid              | gocql.UUID         |
//	uuid, timeuuid              | [16]byte           | raw UUID bytes
//	uuid, timeuuid              | []byte             | raw UUID bytes, length must be 16 bytes
//	uuid, timeuuid              | string             | hex representation, see ParseUUID
//	varint                      | integer types      |
//	varint                      | big.Int            |
//	varint                      | string             | value of number in decimal notation
//	inet                        | net.IP             |
//	inet                        | string             | IPv4 or IPv6 address string
//	tuple                       | slice, array       |
//	tuple                       | struct             | fields are marshaled in order of declaration
//	user-defined type           | gocql.UDTMarshaler | MarshalUDT is called
//	user-defined type           | map[string]interface{} |
//	user-defined type           | struct             | struct fields' cql tags are used for column names
//	date                        | int64              | milliseconds since Unix epoch to start of day (in UTC)
//	date                        | time.Time          | start of day (in UTC)
//	date                        | string             | parsed using "2006-01-02" format
//	duration                    | int64              | duration in nanoseconds
//	duration                    | time.Duration      |
//	duration                    | gocql.Duration     |
//	duration                    | string             | parsed with time.ParseDuration
//
// The marshal/unmarshal error provides a list of supported types when an unsupported type is attempted.

func Marshal(info TypeInfo, value interface{}) ([]byte, error) {
	if info.Version() < protoVersion1 {
		panic("protocol version not set")
	}

	if valueRef := reflect.ValueOf(value); valueRef.Kind() == reflect.Ptr {
		if valueRef.IsNil() {
			return nil, nil
		} else if v, ok := value.(Marshaler); ok {
			return v.MarshalCQL(info)
		} else {
			return Marshal(info, valueRef.Elem().Interface())
		}
	}

	if v, ok := value.(Marshaler); ok {
		return v.MarshalCQL(info)
	}

	switch info.Type() {
	case TypeVarchar:
		return marshalVarchar(value)
	case TypeText:
		return marshalText(value)
	case TypeBlob:
		return marshalBlob(value)
	case TypeAscii:
		return marshalAscii(value)
	case TypeBoolean:
		return marshalBool(value)
	case TypeTinyInt:
		return marshalTinyInt(value)
	case TypeSmallInt:
		return marshalSmallInt(value)
	case TypeInt:
		return marshalInt(value)
	case TypeBigInt:
		return marshalBigInt(value)
	case TypeCounter:
		return marshalCounter(value)
	case TypeFloat:
		return marshalFloat(value)
	case TypeDouble:
		return marshalDouble(value)
	case TypeDecimal:
		return marshalDecimal(value)
	case TypeTime:
		return marshalTime(value)
	case TypeTimestamp:
		return marshalTimestamp(value)
	case TypeList, TypeSet:
		return marshalList(info, value)
	case TypeMap:
		return marshalMap(info, value)
	case TypeUUID:
		return marshalUUID(value)
	case TypeTimeUUID:
		return marshalTimeUUID(value)
	case TypeVarint:
		return marshalVarint(value)
	case TypeInet:
		return marshalInet(value)
	case TypeTuple:
		return marshalTuple(info, value)
	case TypeUDT:
		return marshalUDT(info, value)
	case TypeDate:
		return marshalDate(value)
	case TypeDuration:
		return marshalDuration(value)
	case TypeCustom:
		if vector, ok := info.(VectorType); ok {
			return marshalVector(vector, value)
		}
	}

	// TODO(tux21b): add the remaining types
	return nil, fmt.Errorf("can not marshal %T into %s", value, info)
}

// Unmarshal parses the CQL encoded data based on the info parameter that
// describes the Cassandra internal data type and stores the result in the
// value pointed by value.
//
// If value implements Unmarshaler, it's UnmarshalCQL method is called to
// unmarshal the data.
// If value is a pointer to pointer, it is set to nil if the CQL value is
// null. Otherwise, nulls are unmarshalled as zero value.
//
// Supported conversions are as follows, other type combinations may be added in the future:
//
//	CQL type                                | Go type (value)         | Note
//	varchar, ascii, blob, text              | *string                 |
//	varchar, ascii, blob, text              | *[]byte                 | non-nil buffer is reused
//	bool                                    | *bool                   |
//	tinyint, smallint, int, bigint, counter | *integer types          |
//	tinyint, smallint, int, bigint, counter | *big.Int                |
//	tinyint, smallint, int, bigint, counter | *string                 | formatted as base 10 number
//	float                                   | *float32                |
//	double                                  | *float64                |
//	decimal                                 | *inf.Dec                |
//	time                                    | *int64                  | nanoseconds since start of day
//	time                                    | *time.Duration          |
//	timestamp                               | *int64                  | milliseconds since Unix epoch
//	timestamp                               | *time.Time              |
//	list, set                               | *slice, *array          |
//	map                                     | *map[X]Y                |
//	uuid, timeuuid                          | *string                 | see UUID.String
//	uuid, timeuuid                          | *[]byte                 | raw UUID bytes
//	uuid, timeuuid                          | *gocql.UUID             |
//	timeuuid                                | *time.Time              | timestamp of the UUID
//	inet                                    | *net.IP                 |
//	inet                                    | *string                 | IPv4 or IPv6 address string
//	tuple                                   | *slice, *array          |
//	tuple                                   | *struct                 | struct fields are set in order of declaration
//	user-defined types                      | gocql.UDTUnmarshaler    | UnmarshalUDT is called
//	user-defined types                      | *map[string]interface{} |
//	user-defined types                      | *struct                 | cql tag is used to determine field name
//	date                                    | *time.Time              | time of beginning of the day (in UTC)
//	date                                    | *string                 | formatted with 2006-01-02 format
//	duration                                | *gocql.Duration         |
func Unmarshal(info TypeInfo, data []byte, value interface{}) error {
	if v, ok := value.(Unmarshaler); ok {
		return v.UnmarshalCQL(info, data)
	}

	if isNullableValue(value) {
		return unmarshalNullable(info, data, value)
	}

	switch info.Type() {
	case TypeVarchar:
		return unmarshalVarchar(data, value)
	case TypeText:
		return unmarshalText(data, value)
	case TypeBlob:
		return unmarshalBlob(data, value)
	case TypeAscii:
		return unmarshalAscii(data, value)
	case TypeBoolean:
		return unmarshalBool(data, value)
	case TypeInt:
		return unmarshalInt(data, value)
	case TypeBigInt:
		return unmarshalBigInt(data, value)
	case TypeCounter:
		return unmarshalCounter(data, value)
	case TypeVarint:
		return unmarshalVarint(data, value)
	case TypeSmallInt:
		return unmarshalSmallInt(data, value)
	case TypeTinyInt:
		return unmarshalTinyInt(data, value)
	case TypeFloat:
		return unmarshalFloat(data, value)
	case TypeDouble:
		return unmarshalDouble(data, value)
	case TypeDecimal:
		return unmarshalDecimal(data, value)
	case TypeTime:
		return unmarshalTime(data, value)
	case TypeTimestamp:
		return unmarshalTimestamp(data, value)
	case TypeList, TypeSet:
		return unmarshalList(info, data, value)
	case TypeMap:
		return unmarshalMap(info, data, value)
	case TypeTimeUUID:
		return unmarshalTimeUUID(data, value)
	case TypeUUID:
		return unmarshalUUID(data, value)
	case TypeInet:
		return unmarshalInet(data, value)
	case TypeTuple:
		return unmarshalTuple(info, data, value)
	case TypeUDT:
		return unmarshalUDT(info, data, value)
	case TypeDate:
		return unmarshalDate(data, value)
	case TypeDuration:
		return unmarshalDuration(data, value)
	case TypeCustom:
		if vector, ok := info.(VectorType); ok {
			return unmarshalVector(vector, data, value)
		}
	}

	// TODO(tux21b): add the remaining types
	return fmt.Errorf("can not unmarshal %s into %T", info, value)
}

func isNullableValue(value interface{}) bool {
	v := reflect.ValueOf(value)
	return v.Kind() == reflect.Ptr && v.Type().Elem().Kind() == reflect.Ptr
}

func isNullData(info TypeInfo, data []byte) bool {
	return data == nil
}

func unmarshalNullable(info TypeInfo, data []byte, value interface{}) error {
	valueRef := reflect.ValueOf(value)

	if isNullData(info, data) {
		nilValue := reflect.Zero(valueRef.Type().Elem())
		valueRef.Elem().Set(nilValue)
		return nil
	}

	newValue := reflect.New(valueRef.Type().Elem().Elem())
	valueRef.Elem().Set(newValue)
	return Unmarshal(info, data, newValue.Interface())
}

func marshalVarchar(value interface{}) ([]byte, error) {
	data, err := varchar.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func marshalText(value interface{}) ([]byte, error) {
	data, err := text.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func marshalBlob(value interface{}) ([]byte, error) {
	data, err := blob.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func marshalAscii(value interface{}) ([]byte, error) {
	data, err := ascii.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalVarchar(data []byte, value interface{}) error {
	err := varchar.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func unmarshalText(data []byte, value interface{}) error {
	err := text.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func unmarshalBlob(data []byte, value interface{}) error {
	err := blob.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func unmarshalAscii(data []byte, value interface{}) error {
	err := ascii.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func marshalSmallInt(value interface{}) ([]byte, error) {
	data, err := smallint.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func marshalTinyInt(value interface{}) ([]byte, error) {
	data, err := tinyint.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func marshalInt(value interface{}) ([]byte, error) {
	data, err := cqlint.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func marshalBigInt(value interface{}) ([]byte, error) {
	data, err := bigint.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func marshalCounter(value interface{}) ([]byte, error) {
	data, err := counter.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalCounter(data []byte, value interface{}) error {
	err := counter.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func unmarshalInt(data []byte, value interface{}) error {
	err := cqlint.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func unmarshalBigInt(data []byte, value interface{}) error {
	err := bigint.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func unmarshalSmallInt(data []byte, value interface{}) error {
	err := smallint.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func unmarshalTinyInt(data []byte, value interface{}) error {
	if err := tinyint.Unmarshal(data, value); err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func unmarshalVarint(data []byte, value interface{}) error {
	if err := varint.Unmarshal(data, value); err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func marshalVarint(value interface{}) ([]byte, error) {
	data, err := varint.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func marshalBool(value interface{}) ([]byte, error) {
	data, err := boolean.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalBool(data []byte, value interface{}) error {
	if err := boolean.Unmarshal(data, value); err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func marshalFloat(value interface{}) ([]byte, error) {
	data, err := float.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalFloat(data []byte, value interface{}) error {
	if err := float.Unmarshal(data, value); err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func marshalDouble(value interface{}) ([]byte, error) {
	data, err := double.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalDouble(data []byte, value interface{}) error {
	err := double.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func marshalDecimal(value interface{}) ([]byte, error) {
	data, err := decimal.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalDecimal(data []byte, value interface{}) error {
	if err := decimal.Unmarshal(data, value); err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func marshalTime(value interface{}) ([]byte, error) {
	data, err := cqltime.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalTime(data []byte, value interface{}) error {
	err := cqltime.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func marshalTimestamp(value interface{}) ([]byte, error) {
	data, err := timestamp.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalTimestamp(data []byte, value interface{}) error {
	err := timestamp.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func marshalDate(value interface{}) ([]byte, error) {
	data, err := date.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalDate(data []byte, value interface{}) error {
	err := date.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func marshalDuration(value interface{}) ([]byte, error) {
	switch uv := value.(type) {
	case Duration:
		value = duration.Duration(uv)
	case *Duration:
		value = (*duration.Duration)(uv)
	}
	data, err := duration.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalDuration(data []byte, value interface{}) error {
	switch uv := value.(type) {
	case *Duration:
		value = (*duration.Duration)(uv)
	case **Duration:
		if uv == nil {
			value = (**duration.Duration)(nil)
		} else {
			value = (**duration.Duration)(unsafe.Pointer(uv))
		}
	}
	err := duration.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func writeCollectionSize(info CollectionType, n int, buf *bytes.Buffer) error {
	if n > math.MaxInt32 {
		return marshalErrorf("marshal: collection too large")
	}

	buf.WriteByte(byte(n >> 24))
	buf.WriteByte(byte(n >> 16))
	buf.WriteByte(byte(n >> 8))
	buf.WriteByte(byte(n))

	return nil
}

func marshalList(info TypeInfo, value interface{}) ([]byte, error) {
	listInfo, ok := info.(CollectionType)
	if !ok {
		return nil, marshalErrorf("marshal: can not marshal non collection type into list")
	}

	if value == nil {
		return nil, nil
	} else if _, ok := value.(unsetColumn); ok {
		return nil, nil
	}

	rv := reflect.ValueOf(value)
	t := rv.Type()
	k := t.Kind()
	if k == reflect.Slice && rv.IsNil() {
		return nil, nil
	}

	switch k {
	case reflect.Slice, reflect.Array:
		buf := &bytes.Buffer{}
		n := rv.Len()

		if err := writeCollectionSize(listInfo, n, buf); err != nil {
			return nil, err
		}

		for i := 0; i < n; i++ {
			item, err := Marshal(listInfo.Elem, rv.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			itemLen := len(item)
			// Set the value to null for supported protocols
			if item == nil {
				itemLen = -1
			}
			if err := writeCollectionSize(listInfo, itemLen, buf); err != nil {
				return nil, err
			}
			buf.Write(item)
		}
		return buf.Bytes(), nil
	case reflect.Map:
		elem := t.Elem()
		if elem.Kind() == reflect.Struct && elem.NumField() == 0 {
			rkeys := rv.MapKeys()
			keys := make([]interface{}, len(rkeys))
			for i := 0; i < len(keys); i++ {
				keys[i] = rkeys[i].Interface()
			}
			return marshalList(listInfo, keys)
		}
	}
	return nil, marshalErrorf("can not marshal %T into %s", value, info)
}

func readCollectionSize(info CollectionType, data []byte) (size, read int, err error) {
	if len(data) < 4 {
		return 0, 0, unmarshalErrorf("unmarshal list: unexpected eof")
	}
	size = int(int32(data[0])<<24 | int32(data[1])<<16 | int32(data[2])<<8 | int32(data[3]))
	read = 4
	return
}

func unmarshalList(info TypeInfo, data []byte, value interface{}) error {
	listInfo, ok := info.(CollectionType)
	if !ok {
		return unmarshalErrorf("unmarshal: can not unmarshal none collection type into list")
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Ptr {
		return unmarshalErrorf("can not unmarshal into non-pointer %T", value)
	}
	rv = rv.Elem()
	t := rv.Type()
	k := t.Kind()

	switch k {
	case reflect.Slice, reflect.Array:
		if data == nil {
			if k == reflect.Array {
				return unmarshalErrorf("unmarshal list: can not store nil in array value")
			}
			if rv.IsNil() {
				return nil
			}
			rv.Set(reflect.Zero(t))
			return nil
		}
		n, p, err := readCollectionSize(listInfo, data)
		if err != nil {
			return err
		}
		data = data[p:]
		if k == reflect.Array {
			if rv.Len() != n {
				return unmarshalErrorf("unmarshal list: array with wrong size")
			}
		} else {
			rv.Set(reflect.MakeSlice(t, n, n))
		}
		for i := 0; i < n; i++ {
			m, p, err := readCollectionSize(listInfo, data)
			if err != nil {
				return err
			}
			data = data[p:]
			// In case m < 0, the value is null, and unmarshalData should be nil.
			var unmarshalData []byte
			if m >= 0 {
				if len(data) < m {
					return unmarshalErrorf("unmarshal list: unexpected eof")
				}
				unmarshalData = data[:m]
				data = data[m:]
			}
			if err := Unmarshal(listInfo.Elem, unmarshalData, rv.Index(i).Addr().Interface()); err != nil {
				return err
			}
		}
		return nil
	}
	return unmarshalErrorf("can not unmarshal %s into %T", info, value)
}

func marshalVector(info VectorType, value interface{}) ([]byte, error) {
	if value == nil {
		return nil, nil
	} else if _, ok := value.(unsetColumn); ok {
		return nil, nil
	}

	rv := reflect.ValueOf(value)
	t := rv.Type()
	k := t.Kind()
	if k == reflect.Slice && rv.IsNil() {
		return nil, nil
	}

	switch k {
	case reflect.Slice, reflect.Array:
		buf := &bytes.Buffer{}
		n := rv.Len()
		if n != info.Dimensions {
			return nil, marshalErrorf("expected vector with %d dimensions, received %d", info.Dimensions, n)
		}

		isLengthType := isVectorVariableLengthType(info.SubType)
		for i := 0; i < n; i++ {
			item, err := Marshal(info.SubType, rv.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			if isLengthType {
				writeUnsignedVInt(buf, uint64(len(item)))
			}
			buf.Write(item)
		}
		return buf.Bytes(), nil
	}
	return nil, marshalErrorf("can not marshal %T into %s. Accepted types: slice, array.", value, info)
}

func unmarshalVector(info VectorType, data []byte, value interface{}) error {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Ptr {
		return unmarshalErrorf("can not unmarshal into non-pointer %T", value)
	}
	rv = rv.Elem()
	t := rv.Type()
	if t.Kind() == reflect.Interface {
		if t.NumMethod() != 0 {
			return unmarshalErrorf("can not unmarshal into non-empty interface %T", value)
		}
		t = reflect.TypeOf(info.Zero())
	}

	k := t.Kind()
	switch k {
	case reflect.Slice, reflect.Array:
		if data == nil {
			if k == reflect.Array {
				return unmarshalErrorf("unmarshal vector: can not store nil in array value")
			}
			if rv.IsNil() {
				return nil
			}
			rv.Set(reflect.Zero(t))
			return nil
		}
		if k == reflect.Array {
			if rv.Len() != info.Dimensions {
				return unmarshalErrorf("unmarshal vector: array of size %d cannot store vector of %d dimensions", rv.Len(), info.Dimensions)
			}
		} else {
			rv.Set(reflect.MakeSlice(t, info.Dimensions, info.Dimensions))
			if rv.Kind() == reflect.Interface {
				rv = rv.Elem()
			}
		}
		elemSize := len(data) / info.Dimensions
		isLengthType := isVectorVariableLengthType(info.SubType)
		for i := 0; i < info.Dimensions; i++ {
			offset := 0
			if isLengthType {
				m, p, err := readUnsignedVInt(data)
				if err != nil {
					return err
				}
				elemSize = int(m)
				offset = p
			}
			if offset > 0 {
				data = data[offset:]
			}
			var unmarshalData []byte
			if elemSize >= 0 {
				if len(data) < elemSize {
					return unmarshalErrorf("unmarshal vector: unexpected eof")
				}
				unmarshalData = data[:elemSize]
				data = data[elemSize:]
			}
			err := Unmarshal(info.SubType, unmarshalData, rv.Index(i).Addr().Interface())
			if err != nil {
				return unmarshalErrorf("failed to unmarshal %s into %T: %s", info.SubType, unmarshalData, err.Error())
			}
		}
		return nil
	}
	return unmarshalErrorf("can not unmarshal %s into %T. Accepted types: *slice, *array, *interface{}.", info, value)
}

// isVectorVariableLengthType determines if a type requires explicit length serialization within a vector.
// Variable-length types need their length encoded before the actual data to allow proper deserialization.
// Fixed-length types, on the other hand, don't require this kind of length prefix.
func isVectorVariableLengthType(elemType TypeInfo) bool {
	switch elemType.Type() {
	case TypeVarchar, TypeAscii, TypeBlob, TypeText,
		TypeCounter,
		TypeDuration, TypeDate, TypeTime,
		TypeDecimal, TypeSmallInt, TypeTinyInt, TypeVarint,
		TypeInet,
		TypeList, TypeSet, TypeMap, TypeUDT, TypeTuple:
		return true
	case TypeCustom:
		if vecType, ok := elemType.(VectorType); ok {
			return isVectorVariableLengthType(vecType.SubType)
		}
		return true
	}
	return false
}

func writeUnsignedVInt(buf *bytes.Buffer, v uint64) {
	numBytes := computeUnsignedVIntSize(v)
	if numBytes <= 1 {
		buf.WriteByte(byte(v))
		return
	}

	extraBytes := numBytes - 1
	var tmp = make([]byte, numBytes)
	for i := extraBytes; i >= 0; i-- {
		tmp[i] = byte(v)
		v >>= 8
	}
	tmp[0] |= byte(^(0xff >> uint(extraBytes)))
	buf.Write(tmp)
}

func readUnsignedVInt(data []byte) (uint64, int, error) {
	if len(data) <= 0 {
		return 0, 0, errors.New("unexpected eof")
	}
	firstByte := data[0]
	if firstByte&0x80 == 0 {
		return uint64(firstByte), 1, nil
	}
	numBytes := bits.LeadingZeros32(uint32(^firstByte)) - 24
	ret := uint64(firstByte & (0xff >> uint(numBytes)))
	if len(data) < numBytes+1 {
		return 0, 0, fmt.Errorf("data expect to have %d bytes, but it has only %d", numBytes+1, len(data))
	}
	for i := 0; i < numBytes; i++ {
		ret <<= 8
		ret |= uint64(data[i+1] & 0xff)
	}
	return ret, numBytes + 1, nil
}

func computeUnsignedVIntSize(v uint64) int {
	lead0 := bits.LeadingZeros64(v)
	return (639 - lead0*9) >> 6
}

func marshalMap(info TypeInfo, value interface{}) ([]byte, error) {
	mapInfo, ok := info.(CollectionType)
	if !ok {
		return nil, marshalErrorf("marshal: can not marshal none collection type into map")
	}

	if value == nil {
		return nil, nil
	} else if _, ok := value.(unsetColumn); ok {
		return nil, nil
	}

	rv := reflect.ValueOf(value)

	t := rv.Type()
	if t.Kind() != reflect.Map {
		return nil, marshalErrorf("can not marshal %T into %s", value, info)
	}

	if rv.IsNil() {
		return nil, nil
	}

	buf := &bytes.Buffer{}
	n := rv.Len()

	if err := writeCollectionSize(mapInfo, n, buf); err != nil {
		return nil, err
	}

	keys := rv.MapKeys()
	for _, key := range keys {
		item, err := Marshal(mapInfo.Key, key.Interface())
		if err != nil {
			return nil, err
		}
		itemLen := len(item)
		// Set the key to null for supported protocols
		if item == nil {
			itemLen = -1
		}
		if err := writeCollectionSize(mapInfo, itemLen, buf); err != nil {
			return nil, err
		}
		buf.Write(item)

		item, err = Marshal(mapInfo.Elem, rv.MapIndex(key).Interface())
		if err != nil {
			return nil, err
		}
		itemLen = len(item)
		// Set the value to null for supported protocols
		if item == nil {
			itemLen = -1
		}
		if err := writeCollectionSize(mapInfo, itemLen, buf); err != nil {
			return nil, err
		}
		buf.Write(item)
	}
	return buf.Bytes(), nil
}

func unmarshalMap(info TypeInfo, data []byte, value interface{}) error {
	mapInfo, ok := info.(CollectionType)
	if !ok {
		return unmarshalErrorf("unmarshal: can not unmarshal none collection type into map")
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Ptr {
		return unmarshalErrorf("can not unmarshal into non-pointer %T", value)
	}
	rv = rv.Elem()
	t := rv.Type()
	if t.Kind() != reflect.Map {
		return unmarshalErrorf("can not unmarshal %s into %T", info, value)
	}
	if data == nil {
		rv.Set(reflect.Zero(t))
		return nil
	}
	n, p, err := readCollectionSize(mapInfo, data)
	if err != nil {
		return err
	}
	if n < 0 {
		return unmarshalErrorf("negative map size %d", n)
	}
	rv.Set(reflect.MakeMapWithSize(t, n))
	data = data[p:]
	for i := 0; i < n; i++ {
		m, p, err := readCollectionSize(mapInfo, data)
		if err != nil {
			return err
		}
		data = data[p:]
		key := reflect.New(t.Key())
		// In case m < 0, the key is null, and unmarshalData should be nil.
		var unmarshalData []byte
		if m >= 0 {
			if len(data) < m {
				return unmarshalErrorf("unmarshal map: unexpected eof")
			}
			unmarshalData = data[:m]
			data = data[m:]
		}
		if err := Unmarshal(mapInfo.Key, unmarshalData, key.Interface()); err != nil {
			return err
		}

		m, p, err = readCollectionSize(mapInfo, data)
		if err != nil {
			return err
		}
		data = data[p:]
		val := reflect.New(t.Elem())

		// In case m < 0, the value is null, and unmarshalData should be nil.
		unmarshalData = nil
		if m >= 0 {
			if len(data) < m {
				return unmarshalErrorf("unmarshal map: unexpected eof")
			}
			unmarshalData = data[:m]
			data = data[m:]
		}
		if err := Unmarshal(mapInfo.Elem, unmarshalData, val.Interface()); err != nil {
			return err
		}

		rv.SetMapIndex(key.Elem(), val.Elem())
	}
	return nil
}

func marshalUUID(value interface{}) ([]byte, error) {
	switch uv := value.(type) {
	case UUID:
		value = [16]byte(uv)
	case *UUID:
		value = (*[16]byte)(uv)
	}
	data, err := uuid.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalUUID(data []byte, value interface{}) error {
	switch uv := value.(type) {
	case *UUID:
		value = (*[16]byte)(uv)
	case **UUID:
		if uv == nil {
			value = (**[16]byte)(nil)
		} else {
			value = (**[16]byte)(unsafe.Pointer(uv))
		}
	}
	err := uuid.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func marshalTimeUUID(value interface{}) ([]byte, error) {
	switch uv := value.(type) {
	case UUID:
		value = [16]byte(uv)
	case *UUID:
		value = (*[16]byte)(uv)
	}
	data, err := timeuuid.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalTimeUUID(data []byte, value interface{}) error {
	switch uv := value.(type) {
	case *UUID:
		value = (*[16]byte)(uv)
	case **UUID:
		if uv == nil {
			value = (**[16]byte)(nil)
		} else {
			value = (**[16]byte)(unsafe.Pointer(uv))
		}
	}
	err := timeuuid.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func marshalInet(value interface{}) ([]byte, error) {
	data, err := inet.Marshal(value)
	if err != nil {
		return nil, wrapMarshalError(err, "marshal error")
	}
	return data, nil
}

func unmarshalInet(data []byte, value interface{}) error {
	err := inet.Unmarshal(data, value)
	if err != nil {
		return wrapUnmarshalError(err, "unmarshal error")
	}
	return nil
}

func marshalTuple(info TypeInfo, value interface{}) ([]byte, error) {
	tuple := info.(TupleTypeInfo)
	switch v := value.(type) {
	case unsetColumn:
		return nil, unmarshalErrorf("Invalid request: UnsetValue is unsupported for tuples")
	case []interface{}:
		if len(v) != len(tuple.Elems) {
			return nil, unmarshalErrorf("cannont marshal tuple: wrong number of elements")
		}

		var buf []byte
		for i, elem := range v {
			if elem == nil {
				buf = appendIntNeg1(buf)
				continue
			}

			data, err := Marshal(tuple.Elems[i], elem)
			if err != nil {
				return nil, err
			}

			n := len(data)
			buf = appendInt(buf, int32(n))
			buf = append(buf, data...)
		}

		return buf, nil
	}

	rv := reflect.ValueOf(value)
	t := rv.Type()
	k := t.Kind()

	switch k {
	case reflect.Struct:
		if v := t.NumField(); v != len(tuple.Elems) {
			return nil, marshalErrorf("can not marshal tuple into struct %v, not enough fields have %d need %d", t, v, len(tuple.Elems))
		}

		var buf []byte
		for i, elem := range tuple.Elems {
			field := rv.Field(i)

			if field.Kind() == reflect.Ptr && field.IsNil() {
				buf = appendIntNeg1(buf)
				continue
			}

			data, err := Marshal(elem, field.Interface())
			if err != nil {
				return nil, err
			}

			n := len(data)
			buf = appendInt(buf, int32(n))
			buf = append(buf, data...)
		}

		return buf, nil
	case reflect.Slice, reflect.Array:
		size := rv.Len()
		if size != len(tuple.Elems) {
			return nil, marshalErrorf("can not marshal tuple into %v of length %d need %d elements", k, size, len(tuple.Elems))
		}

		var buf []byte
		for i, elem := range tuple.Elems {
			item := rv.Index(i)

			if item.Kind() == reflect.Ptr && item.IsNil() {
				buf = appendIntNeg1(buf)
				continue
			}

			data, err := Marshal(elem, item.Interface())
			if err != nil {
				return nil, err
			}

			n := len(data)
			buf = appendInt(buf, int32(n))
			buf = append(buf, data...)
		}

		return buf, nil
	}

	return nil, marshalErrorf("cannot marshal %T into %s", value, tuple)
}

func readBytes(p []byte) ([]byte, []byte) {
	// TODO: really should use a framer
	size := readInt(p)
	p = p[4:]
	if size < 0 {
		return nil, p
	}
	return p[:size], p[size:]
}

// currently only support unmarshal into a list of values, this makes it possible
// to support tuples without changing the query API. In the future this can be extend
// to allow unmarshalling into custom tuple types.
func unmarshalTuple(info TypeInfo, data []byte, value interface{}) error {
	if v, ok := value.(Unmarshaler); ok {
		return v.UnmarshalCQL(info, data)
	}

	tuple := info.(TupleTypeInfo)
	switch v := value.(type) {
	case []interface{}:
		for i, elem := range tuple.Elems {
			// each element inside data is a [bytes]
			var p []byte
			if len(data) >= 4 {
				p, data = readBytes(data)
			}
			err := Unmarshal(elem, p, v[i])
			if err != nil {
				return err
			}
		}

		return nil
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Ptr {
		return unmarshalErrorf("can not unmarshal into non-pointer %T", value)
	}

	rv = rv.Elem()
	t := rv.Type()
	k := t.Kind()

	switch k {
	case reflect.Struct:
		if v := t.NumField(); v != len(tuple.Elems) {
			return unmarshalErrorf("can not unmarshal tuple into struct %v, not enough fields have %d need %d", t, v, len(tuple.Elems))
		}

		for i, elem := range tuple.Elems {
			var p []byte
			if len(data) >= 4 {
				p, data = readBytes(data)
			}

			v, err := elem.NewWithError()
			if err != nil {
				return err
			}
			if err := Unmarshal(elem, p, v); err != nil {
				return err
			}

			switch rv.Field(i).Kind() {
			case reflect.Ptr:
				if p != nil {
					rv.Field(i).Set(reflect.ValueOf(v))
				} else {
					rv.Field(i).Set(reflect.Zero(reflect.TypeOf(v)))
				}
			default:
				rv.Field(i).Set(reflect.ValueOf(v).Elem())
			}
		}

		return nil
	case reflect.Slice, reflect.Array:
		if k == reflect.Array {
			size := rv.Len()
			if size != len(tuple.Elems) {
				return unmarshalErrorf("can not unmarshal tuple into array of length %d need %d elements", size, len(tuple.Elems))
			}
		} else {
			rv.Set(reflect.MakeSlice(t, len(tuple.Elems), len(tuple.Elems)))
		}

		for i, elem := range tuple.Elems {
			var p []byte
			if len(data) >= 4 {
				p, data = readBytes(data)
			}

			v, err := elem.NewWithError()
			if err != nil {
				return err
			}
			if err := Unmarshal(elem, p, v); err != nil {
				return err
			}

			switch rv.Index(i).Kind() {
			case reflect.Ptr:
				if p != nil {
					rv.Index(i).Set(reflect.ValueOf(v))
				} else {
					rv.Index(i).Set(reflect.Zero(reflect.TypeOf(v)))
				}
			default:
				rv.Index(i).Set(reflect.ValueOf(v).Elem())
			}
		}

		return nil
	}

	return unmarshalErrorf("cannot unmarshal %s into %T", info, value)
}

// UDTMarshaler is an interface which should be implemented by users wishing to
// handle encoding UDT types to sent to Cassandra. Note: due to current implentations
// methods defined for this interface must be value receivers not pointer receivers.
type UDTMarshaler interface {
	// MarshalUDT will be called for each field in the the UDT returned by Cassandra,
	// the implementor should marshal the type to return by for example calling
	// Marshal.
	MarshalUDT(name string, info TypeInfo) ([]byte, error)
}

// UDTUnmarshaler should be implemented by users wanting to implement custom
// UDT unmarshaling.
type UDTUnmarshaler interface {
	// UnmarshalUDT will be called for each field in the UDT return by Cassandra,
	// the implementor should unmarshal the data into the value of their chosing,
	// for example by calling Unmarshal.
	UnmarshalUDT(name string, info TypeInfo, data []byte) error
}

func marshalUDT(info TypeInfo, value interface{}) ([]byte, error) {
	udt := info.(UDTTypeInfo)

	switch v := value.(type) {
	case Marshaler:
		return v.MarshalCQL(info)
	case unsetColumn:
		return nil, unmarshalErrorf("invalid request: UnsetValue is unsupported for user defined types")
	case UDTMarshaler:
		var buf []byte
		for _, e := range udt.Elements {
			data, err := v.MarshalUDT(e.Name, e.Type)
			if err != nil {
				return nil, err
			}

			buf = appendBytes(buf, data)
		}

		return buf, nil
	case map[string]interface{}:
		var buf []byte
		for _, e := range udt.Elements {
			val, ok := v[e.Name]
			var data []byte

			if ok {
				var err error
				data, err = Marshal(e.Type, val)
				if err != nil {
					return nil, err
				}
			}

			buf = appendBytes(buf, data)
		}

		return buf, nil
	}

	k := reflect.ValueOf(value)
	if k.Kind() == reflect.Ptr {
		if k.IsNil() {
			return nil, marshalErrorf("cannot marshal %T into %s", value, info)
		}
		k = k.Elem()
	}

	if k.Kind() != reflect.Struct || !k.IsValid() {
		return nil, marshalErrorf("cannot marshal %T into %s", value, info)
	}

	fields := make(map[string]reflect.Value)
	t := reflect.TypeOf(value)
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)

		if tag := sf.Tag.Get("cql"); tag != "" {
			fields[tag] = k.Field(i)
		}
	}

	var buf []byte
	for _, e := range udt.Elements {
		f, ok := fields[e.Name]
		if !ok {
			f = k.FieldByName(e.Name)
		}

		var data []byte
		if f.IsValid() && f.CanInterface() {
			var err error
			data, err = Marshal(e.Type, f.Interface())
			if err != nil {
				return nil, err
			}
		}

		buf = appendBytes(buf, data)
	}

	return buf, nil
}

func unmarshalUDT(info TypeInfo, data []byte, value interface{}) error {
	switch v := value.(type) {
	case Unmarshaler:
		return v.UnmarshalCQL(info, data)
	case UDTUnmarshaler:
		udt := info.(UDTTypeInfo)

		for id, e := range udt.Elements {
			if len(data) == 0 {
				return nil
			}
			if len(data) < 4 {
				return unmarshalErrorf("can not unmarshal %s: field [%d]%s: unexpected eof", info, id, e.Name)
			}

			var p []byte
			p, data = readBytes(data)
			if err := v.UnmarshalUDT(e.Name, e.Type, p); err != nil {
				return err
			}
		}

		return nil
	case *map[string]interface{}:
		udt := info.(UDTTypeInfo)

		rv := reflect.ValueOf(value)
		if rv.Kind() != reflect.Ptr {
			return unmarshalErrorf("can not unmarshal into non-pointer %T", value)
		}

		rv = rv.Elem()
		t := rv.Type()
		if t.Kind() != reflect.Map {
			return unmarshalErrorf("can not unmarshal %s into %T", info, value)
		} else if data == nil {
			rv.Set(reflect.Zero(t))
			return nil
		}

		rv.Set(reflect.MakeMap(t))
		m := *v

		for id, e := range udt.Elements {
			if len(data) == 0 {
				return nil
			}
			if len(data) < 4 {
				return unmarshalErrorf("can not unmarshal %s: field [%d]%s: unexpected eof", info, id, e.Name)
			}

			valType, err := goType(e.Type)
			if err != nil {
				return unmarshalErrorf("can not unmarshal %s: %v", info, err)
			}

			val := reflect.New(valType)

			var p []byte
			p, data = readBytes(data)

			if err := Unmarshal(e.Type, p, val.Interface()); err != nil {
				return err
			}

			m[e.Name] = val.Elem().Interface()
		}

		return nil
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Ptr {
		return unmarshalErrorf("can not unmarshal into non-pointer %T", value)
	}
	k := rv.Elem()
	if k.Kind() != reflect.Struct || !k.IsValid() {
		return unmarshalErrorf("cannot unmarshal %s into %T", info, value)
	}

	if len(data) == 0 {
		if k.CanSet() {
			k.Set(reflect.Zero(k.Type()))
		}

		return nil
	}

	t := k.Type()
	fields := make(map[string]reflect.Value, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)

		if tag := sf.Tag.Get("cql"); tag != "" {
			fields[tag] = k.Field(i)
		}
	}

	udt := info.(UDTTypeInfo)
	for id, e := range udt.Elements {
		if len(data) == 0 {
			return nil
		}
		if len(data) < 4 {
			// UDT def does not match the column value
			return unmarshalErrorf("can not unmarshal %s: field [%d]%s: unexpected eof", info, id, e.Name)
		}

		var p []byte
		p, data = readBytes(data)

		f, ok := fields[e.Name]
		if !ok {
			f = k.FieldByName(e.Name)
			if f == emptyValue { //nolint:govet // no other way to do that
				// skip fields which exist in the UDT but not in
				// the struct passed in
				continue
			}
		}

		if !f.IsValid() || !f.CanAddr() {
			return unmarshalErrorf("cannot unmarshal %s into %T: field %v is not valid", info, value, e.Name)
		}

		fk := f.Addr().Interface()
		if err := Unmarshal(e.Type, p, fk); err != nil {
			return err
		}
	}

	return nil
}

// TypeInfo describes a Cassandra specific data type.
type TypeInfo interface {
	Type() Type
	Version() byte
	Custom() string

	// NewWithError creates a pointer to an empty version of whatever type
	// is referenced by the TypeInfo receiver.
	//
	// If there is no corresponding Go type for the CQL type, NewWithError returns an error.
	NewWithError() (interface{}, error)
}

type NativeType struct {
	//only used for TypeCustom
	custom string
	typ    Type
	proto  byte
}

func NewNativeType(proto byte, typ Type) NativeType {
	return NativeType{proto: proto, typ: typ, custom: ""}
}

func NewCustomType(proto byte, typ Type, custom string) NativeType {
	return NativeType{proto: proto, typ: typ, custom: custom}
}

func (t NativeType) NewWithError() (interface{}, error) {
	typ, err := goType(t)
	if err != nil {
		return nil, err
	}
	return reflect.New(typ).Interface(), nil
}

func (t NativeType) Type() Type {
	return t.typ
}

func (t NativeType) Version() byte {
	return t.proto
}

func (t NativeType) Custom() string {
	return t.custom
}

func (t NativeType) String() string {
	switch t.typ {
	case TypeCustom:
		return fmt.Sprintf("%s(%s)", t.typ, t.custom)
	default:
		return t.typ.String()
	}
}

func NewCollectionType(m NativeType, key, elem TypeInfo) CollectionType {
	return CollectionType{
		NativeType: m,
		Key:        key,
		Elem:       elem,
	}
}

type CollectionType struct {
	// Key is used only for TypeMap
	Key TypeInfo
	// Elem is used for TypeMap, TypeList and TypeSet
	Elem TypeInfo
	NativeType
}

type VectorType struct {
	SubType TypeInfo
	NativeType
	Dimensions int
}

// Zero returns the zero value for the vector CQL type.
func (v VectorType) Zero() interface{} {
	t, e := v.SubType.NewWithError()
	if e != nil {
		return nil
	}
	return reflect.Zero(reflect.SliceOf(reflect.TypeOf(t))).Interface()
}

func (t CollectionType) NewWithError() (interface{}, error) {
	typ, err := goType(t)
	if err != nil {
		return nil, err
	}
	return reflect.New(typ).Interface(), nil
}

func (t CollectionType) String() string {
	switch t.typ {
	case TypeMap:
		return fmt.Sprintf("%s(%s, %s)", t.typ, t.Key, t.Elem)
	case TypeList, TypeSet:
		return fmt.Sprintf("%s(%s)", t.typ, t.Elem)
	case TypeCustom:
		return fmt.Sprintf("%s(%s)", t.typ, t.custom)
	default:
		return t.typ.String()
	}
}

func NewTupleType(n NativeType, elems ...TypeInfo) TupleTypeInfo {
	return TupleTypeInfo{
		NativeType: n,
		Elems:      elems,
	}
}

type TupleTypeInfo struct {
	Elems []TypeInfo
	NativeType
}

func (t TupleTypeInfo) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s(", t.typ))
	for _, elem := range t.Elems {
		buf.WriteString(fmt.Sprintf("%s, ", elem))
	}
	buf.Truncate(buf.Len() - 2)
	buf.WriteByte(')')
	return buf.String()
}

func (t TupleTypeInfo) NewWithError() (interface{}, error) {
	typ, err := goType(t)
	if err != nil {
		return nil, err
	}
	return reflect.New(typ).Interface(), nil
}

type UDTField struct {
	Type TypeInfo
	Name string
}

func NewUDTType(proto byte, name, keySpace string, elems ...UDTField) UDTTypeInfo {
	return UDTTypeInfo{
		NativeType: NativeType{proto: proto, typ: TypeUDT, custom: ""},
		Name:       name,
		KeySpace:   keySpace,
		Elements:   elems,
	}
}

type UDTTypeInfo struct {
	KeySpace string
	Name     string
	Elements []UDTField
	NativeType
}

func (t UDTTypeInfo) NewWithError() (interface{}, error) {
	typ, err := goType(t)
	if err != nil {
		return nil, err
	}
	return reflect.New(typ).Interface(), nil
}

func (t UDTTypeInfo) String() string {
	buf := &bytes.Buffer{}

	fmt.Fprintf(buf, "%s.%s{", t.KeySpace, t.Name)
	first := true
	for _, e := range t.Elements {
		if !first {
			fmt.Fprint(buf, ",")
		} else {
			first = false
		}

		fmt.Fprintf(buf, "%s=%v", e.Name, e.Type)
	}
	fmt.Fprint(buf, "}")

	return buf.String()
}

// String returns a human readable name for the Cassandra datatype
// described by t.
// Type is the identifier of a Cassandra internal datatype.
type Type int

const (
	TypeCustom    Type = 0x0000
	TypeAscii     Type = 0x0001
	TypeBigInt    Type = 0x0002
	TypeBlob      Type = 0x0003
	TypeBoolean   Type = 0x0004
	TypeCounter   Type = 0x0005
	TypeDecimal   Type = 0x0006
	TypeDouble    Type = 0x0007
	TypeFloat     Type = 0x0008
	TypeInt       Type = 0x0009
	TypeText      Type = 0x000A
	TypeTimestamp Type = 0x000B
	TypeUUID      Type = 0x000C
	TypeVarchar   Type = 0x000D
	TypeVarint    Type = 0x000E
	TypeTimeUUID  Type = 0x000F
	TypeInet      Type = 0x0010
	TypeDate      Type = 0x0011
	TypeTime      Type = 0x0012
	TypeSmallInt  Type = 0x0013
	TypeTinyInt   Type = 0x0014
	TypeDuration  Type = 0x0015
	TypeList      Type = 0x0020
	TypeMap       Type = 0x0021
	TypeSet       Type = 0x0022
	TypeUDT       Type = 0x0030
	TypeTuple     Type = 0x0031
)

// String returns the name of the identifier.
func (t Type) String() string {
	switch t {
	case TypeCustom:
		return "custom"
	case TypeAscii:
		return "ascii"
	case TypeBigInt:
		return "bigint"
	case TypeBlob:
		return "blob"
	case TypeBoolean:
		return "boolean"
	case TypeCounter:
		return "counter"
	case TypeDecimal:
		return "decimal"
	case TypeDouble:
		return "double"
	case TypeFloat:
		return "float"
	case TypeInt:
		return "int"
	case TypeText:
		return "text"
	case TypeTimestamp:
		return "timestamp"
	case TypeUUID:
		return "uuid"
	case TypeVarchar:
		return "varchar"
	case TypeTimeUUID:
		return "timeuuid"
	case TypeInet:
		return "inet"
	case TypeDate:
		return "date"
	case TypeDuration:
		return "duration"
	case TypeTime:
		return "time"
	case TypeSmallInt:
		return "smallint"
	case TypeTinyInt:
		return "tinyint"
	case TypeList:
		return "list"
	case TypeMap:
		return "map"
	case TypeSet:
		return "set"
	case TypeVarint:
		return "varint"
	case TypeTuple:
		return "tuple"
	default:
		return fmt.Sprintf("unknown_type_%d", t)
	}
}

type MarshalError struct {
	cause error
	msg   string
}

func (m MarshalError) Error() string {
	if m.cause != nil {
		return m.msg + ": " + m.cause.Error()
	}
	return m.msg
}

func (m MarshalError) Cause() error { return m.cause }

func (m MarshalError) Unwrap() error {
	return m.cause
}

func marshalErrorf(format string, args ...interface{}) MarshalError {
	return MarshalError{msg: fmt.Sprintf(format, args...)}
}

func wrapMarshalError(err error, msg string) MarshalError {
	return MarshalError{msg: msg, cause: err}
}

func wrapMarshalErrorf(err error, format string, a ...interface{}) MarshalError {
	return MarshalError{msg: fmt.Sprintf(format, a...), cause: err}
}

type UnmarshalError struct {
	cause error
	msg   string
}

func (m UnmarshalError) Error() string {
	if m.cause != nil {
		return m.msg + ": " + m.cause.Error()
	}
	return m.msg
}

func (m UnmarshalError) Cause() error { return m.cause }

func (m UnmarshalError) Unwrap() error {
	return m.cause
}

func unmarshalErrorf(format string, args ...interface{}) UnmarshalError {
	return UnmarshalError{msg: fmt.Sprintf(format, args...)}
}

func wrapUnmarshalError(err error, msg string) UnmarshalError {
	return UnmarshalError{msg: msg, cause: err}
}

func wrapUnmarshalErrorf(err error, format string, a ...interface{}) UnmarshalError {
	return UnmarshalError{msg: fmt.Sprintf(format, a...), cause: err}
}
