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
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/inf.v0"
)

type RowData struct {
	Columns []string
	Values  []any
}

// asVectorType attempts to convert a NativeType(custom) which represents a VectorType
// into a concrete VectorType. It also works recursively (nested vectors).
func asVectorType(t TypeInfo) (VectorType, bool) {
	if v, ok := t.(VectorType); ok {
		return v, true
	}
	n, ok := t.(NativeType)
	if !ok || n.Type() != TypeCustom {
		return VectorType{}, false
	}
	const vectorTypePrefix = apacheCassandraTypePrefix + "VectorType"
	if !strings.HasPrefix(n.Custom(), vectorTypePrefix+"(") {
		return VectorType{}, false
	}

	spec := strings.TrimPrefix(n.Custom(), vectorTypePrefix)
	spec = strings.Trim(spec, "()")
	// split last comma -> subtype spec , dimensions
	idx := strings.LastIndex(spec, ",")
	if idx <= 0 {
		return VectorType{}, false
	}
	subStr := strings.TrimSpace(spec[:idx])
	dimStr := strings.TrimSpace(spec[idx+1:])
	dim, err := strconv.Atoi(dimStr)
	if err != nil {
		return VectorType{}, false
	}
	subType := getCassandraLongType(subStr, n.Version(), nopLogger{})
	// recurse if subtype itself is still a custom vector
	if innerVec, ok := asVectorType(subType); ok {
		subType = innerVec
	}
	return VectorType{
		NativeType: NewCustomType(n.Version(), TypeCustom, vectorTypePrefix),
		SubType:    subType,
		Dimensions: dim,
	}, true
}

func goType(t TypeInfo) (reflect.Type, error) {
	switch t.Type() {
	case TypeVarchar, TypeAscii, TypeInet, TypeText:
		return reflect.TypeOf(*new(string)), nil
	case TypeBigInt, TypeCounter:
		return reflect.TypeOf(*new(int64)), nil
	case TypeTime:
		return reflect.TypeOf(*new(time.Duration)), nil
	case TypeTimestamp:
		return reflect.TypeOf(*new(time.Time)), nil
	case TypeBlob:
		return reflect.TypeOf(*new([]byte)), nil
	case TypeBoolean:
		return reflect.TypeOf(*new(bool)), nil
	case TypeFloat:
		return reflect.TypeOf(*new(float32)), nil
	case TypeDouble:
		return reflect.TypeOf(*new(float64)), nil
	case TypeInt:
		return reflect.TypeOf(*new(int)), nil
	case TypeSmallInt:
		return reflect.TypeOf(*new(int16)), nil
	case TypeTinyInt:
		return reflect.TypeOf(*new(int8)), nil
	case TypeDecimal:
		return reflect.TypeOf(*new(*inf.Dec)), nil
	case TypeUUID, TypeTimeUUID:
		return reflect.TypeOf(*new(UUID)), nil
	case TypeList, TypeSet:
		elemType, err := goType(t.(CollectionType).Elem)
		if err != nil {
			return nil, err
		}
		return reflect.SliceOf(elemType), nil
	case TypeMap:
		keyType, err := goType(t.(CollectionType).Key)
		if err != nil {
			return nil, err
		}
		valueType, err := goType(t.(CollectionType).Elem)
		if err != nil {
			return nil, err
		}
		return reflect.MapOf(keyType, valueType), nil
	case TypeVarint:
		return reflect.TypeOf(*new(*big.Int)), nil
	case TypeTuple:
		// what can we do here? all there is to do is to make a list of any
		tuple := t.(TupleTypeInfo)
		return reflect.TypeOf(make([]any, len(tuple.Elems))), nil
	case TypeUDT:
		return reflect.TypeOf(make(map[string]any)), nil
	case TypeDate:
		return reflect.TypeOf(*new(time.Time)), nil
	case TypeDuration:
		return reflect.TypeOf(*new(Duration)), nil
	case TypeCustom:
		// Handle VectorType encoded as custom
		if vec, ok := asVectorType(t); ok {
			innerPtr, err := vec.SubType.NewWithError()
			if err != nil {
				return nil, err
			}
			elemType := reflect.TypeOf(innerPtr)
			if elemType.Kind() == reflect.Ptr {
				elemType = elemType.Elem()
			}
			return reflect.SliceOf(elemType), nil
		}
		return nil, fmt.Errorf("cannot create Go type for unknown CQL type %s", t)
	default:
		return nil, fmt.Errorf("cannot create Go type for unknown CQL type %s", t)
	}
}

func dereference(i any) any {
	// Fast path: avoid reflect for the common pointer types returned by
	// NativeType.NewWithError and used in RowData/MapScan.
	switch v := i.(type) {
	case *string:
		return *v
	case *int:
		return *v
	case *int64:
		return *v
	case *int32:
		return *v
	case *int16:
		return *v
	case *int8:
		return *v
	case *float64:
		return *v
	case *float32:
		return *v
	case *bool:
		return *v
	case *[]byte:
		return *v
	case *time.Time:
		return *v
	case *time.Duration:
		return *v
	case *UUID:
		return *v
	case *Duration:
		return *v
	case *inf.Dec:
		return *v
	case *big.Int:
		return *v
	case *[]any:
		return *v
	case *map[string]any:
		return *v
	default:
		return reflect.Indirect(reflect.ValueOf(i)).Interface()
	}
}

// TODO: Cover with unit tests.
// Parses long Java-style type definition to internal data structures.
func getCassandraLongType(name string, protoVer byte, logger StdLogger) TypeInfo {
	const prefix = apacheCassandraTypePrefix
	if strings.HasPrefix(name, prefix+"SetType") {
		return CollectionType{
			NativeType: NewNativeType(protoVer, TypeSet),
			Elem:       getCassandraLongType(unwrapCompositeTypeDefinition(name, prefix+"SetType", '('), protoVer, logger),
		}
	} else if strings.HasPrefix(name, prefix+"ListType") {
		return CollectionType{
			NativeType: NewNativeType(protoVer, TypeList),
			Elem:       getCassandraLongType(unwrapCompositeTypeDefinition(name, prefix+"ListType", '('), protoVer, logger),
		}
	} else if strings.HasPrefix(name, prefix+"MapType") {
		names := splitJavaCompositeTypes(name, prefix+"MapType")
		if len(names) != 2 {
			logger.Printf("gocql: error parsing map type, it has %d subelements, expecting 2\n", len(names))
			return NewNativeType(protoVer, TypeCustom)
		}
		return CollectionType{
			NativeType: NewNativeType(protoVer, TypeMap),
			Key:        getCassandraLongType(names[0], protoVer, logger),
			Elem:       getCassandraLongType(names[1], protoVer, logger),
		}
	} else if strings.HasPrefix(name, prefix+"TupleType") {
		names := splitJavaCompositeTypes(name, prefix+"TupleType")
		types := make([]TypeInfo, len(names))

		for i, name := range names {
			types[i] = getCassandraLongType(name, protoVer, logger)
		}

		return TupleTypeInfo{
			NativeType: NewNativeType(protoVer, TypeTuple),
			Elems:      types,
		}
	} else if strings.HasPrefix(name, prefix+"UserType") {
		names := splitJavaCompositeTypes(name, prefix+"UserType")
		fields := make([]UDTField, len(names)-2)

		for i := 2; i < len(names); i++ {
			spec := strings.Split(names[i], ":")
			fieldName, _ := hex.DecodeString(spec[0])
			fields[i-2] = UDTField{
				Name: string(fieldName),
				Type: getCassandraLongType(spec[1], protoVer, logger),
			}
		}

		udtName, _ := hex.DecodeString(names[1])
		return UDTTypeInfo{
			NativeType: NewNativeType(protoVer, TypeUDT),
			KeySpace:   names[0],
			Name:       string(udtName),
			Elements:   fields,
		}
	} else if strings.HasPrefix(name, prefix+"VectorType") {
		names := splitJavaCompositeTypes(name, prefix+"VectorType")
		subType := getCassandraLongType(strings.TrimSpace(names[0]), protoVer, logger)
		dim, err := strconv.Atoi(strings.TrimSpace(names[1]))
		if err != nil {
			logger.Printf("gocql: error parsing vector dimensions: %v\n", err)
			return NewNativeType(protoVer, TypeCustom)
		}

		return VectorType{
			NativeType: NewCustomType(protoVer, TypeCustom, prefix+"VectorType"),
			SubType:    subType,
			Dimensions: dim,
		}
	} else if strings.HasPrefix(name, prefix+"FrozenType") {
		names := splitJavaCompositeTypes(name, prefix+"FrozenType")
		return getCassandraLongType(strings.TrimSpace(names[0]), protoVer, logger)
	} else {
		// basic type
		return NativeType{
			proto: protoVer,
			typ:   getApacheCassandraType(name),
		}
	}
}

func splitJavaCompositeTypes(name string, typeName string) []string {
	return splitCompositeTypes(name, typeName, '(', ')')
}

func unwrapCompositeTypeDefinition(name string, typeName string, typeOpen int32) string {
	return strings.TrimPrefix(name[:len(name)-1], typeName+string(typeOpen))
}

func splitCompositeTypes(name string, typeName string, typeOpen int32, typeClose int32) []string {
	def := unwrapCompositeTypeDefinition(name, typeName, typeOpen)
	if !strings.Contains(def, string(typeOpen)) {
		parts := strings.Split(def, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	var parts []string
	lessCount := 0
	segment := ""
	for _, char := range def {
		if char == ',' && lessCount == 0 {
			if segment != "" {
				parts = append(parts, strings.TrimSpace(segment))
			}
			segment = ""
			continue
		}
		segment += string(char)
		if char == typeOpen {
			lessCount++
		} else if char == typeClose {
			lessCount--
		}
	}
	if segment != "" {
		parts = append(parts, strings.TrimSpace(segment))
	}
	return parts
}

func getApacheCassandraType(class string) Type {
	switch strings.TrimPrefix(class, apacheCassandraTypePrefix) {
	case "AsciiType":
		return TypeAscii
	case "LongType":
		return TypeBigInt
	case "BytesType":
		return TypeBlob
	case "BooleanType":
		return TypeBoolean
	case "CounterColumnType":
		return TypeCounter
	case "DecimalType":
		return TypeDecimal
	case "DoubleType":
		return TypeDouble
	case "FloatType":
		return TypeFloat
	case "Int32Type":
		return TypeInt
	case "ShortType":
		return TypeSmallInt
	case "ByteType":
		return TypeTinyInt
	case "TimeType":
		return TypeTime
	case "DateType", "TimestampType":
		return TypeTimestamp
	case "UUIDType", "LexicalUUIDType":
		return TypeUUID
	case "UTF8Type":
		return TypeVarchar
	case "IntegerType":
		return TypeVarint
	case "TimeUUIDType":
		return TypeTimeUUID
	case "InetAddressType":
		return TypeInet
	case "MapType":
		return TypeMap
	case "ListType":
		return TypeList
	case "SetType":
		return TypeSet
	case "TupleType":
		return TypeTuple
	case "DurationType":
		return TypeDuration
	case "SimpleDateType":
		return TypeDate
	case "UserType":
		return TypeUDT
	default:
		return TypeCustom
	}
}

func (r *RowData) rowMap(m map[string]any) {
	for i, column := range r.Columns {
		val := dereference(r.Values[i])
		if valVal := reflect.ValueOf(val); valVal.Kind() == reflect.Slice && !valVal.IsNil() {
			valCopy := reflect.MakeSlice(valVal.Type(), valVal.Len(), valVal.Cap())
			reflect.Copy(valCopy, valVal)
			m[column] = valCopy.Interface()
		} else {
			m[column] = val
		}
	}
}

// TupeColumnName will return the column name of a tuple value in a column named
// c at index n. It should be used if a specific element within a tuple is needed
// to be extracted from a map returned from SliceMap or MapScan.
func TupleColumnName(c string, n int) string {
	return fmt.Sprintf("%s[%d]", c, n)
}

// RowData returns the RowData for the iterator.
func (iter *Iter) RowData() (RowData, error) {
	if iter.err != nil {
		return RowData{}, iter.err
	}

	columns, err := iter.getScanColumns()
	if err != nil {
		return RowData{}, err
	}

	values, err := iter.newScanValues()
	if err != nil {
		return RowData{}, err
	}

	return RowData{
		Columns: columns,
		Values:  values,
	}, nil
}

// getScanColumns returns the cached column names for this iterator,
// computing them on the first call. Column names don't change between
// rows, so they are computed once and reused.
//
// The returned slice is shared across all callers and must not be mutated.
func (iter *Iter) getScanColumns() ([]string, error) {
	if iter.scanColumns != nil {
		return iter.scanColumns, nil
	}

	actualSize := iter.meta.actualColCount
	columns := make([]string, actualSize)
	idx := 0
	for _, column := range iter.Columns() {
		if c, ok := column.TypeInfo.(TupleTypeInfo); !ok {
			if idx >= actualSize {
				err := fmt.Errorf("gocql: column count overflow in RowData: metadata predicted %d columns but encountered more", actualSize)
				iter.err = err
				return nil, err
			}
			columns[idx] = column.Name
			idx++
		} else {
			for i := range c.Elems {
				if idx >= actualSize {
					err := fmt.Errorf("gocql: column count overflow in RowData: metadata predicted %d columns but encountered more", actualSize)
					iter.err = err
					return nil, err
				}
				columns[idx] = TupleColumnName(column.Name, i)
				idx++
			}
		}
	}

	if idx != actualSize {
		err := fmt.Errorf("gocql: column count mismatch in RowData: metadata predicted %d columns but got %d", actualSize, idx)
		iter.err = err
		return nil, err
	}

	iter.scanColumns = columns
	return columns, nil
}

// newScanValues allocates fresh zero-value pointers for each column,
// suitable for passing to Scan. Values must be freshly allocated each
// call because Scan mutates them.
func (iter *Iter) newScanValues() ([]any, error) {
	actualSize := iter.meta.actualColCount
	values := make([]any, actualSize)
	idx := 0
	for _, column := range iter.Columns() {
		if c, ok := column.TypeInfo.(TupleTypeInfo); !ok {
			if idx >= actualSize {
				err := fmt.Errorf("gocql: column count overflow in newScanValues: metadata predicted %d columns but encountered more", actualSize)
				iter.err = err
				return nil, err
			}
			val, err := column.TypeInfo.NewWithError()
			if err != nil {
				iter.err = err
				return nil, err
			}
			values[idx] = val
			idx++
		} else {
			for _, elem := range c.Elems {
				if idx >= actualSize {
					err := fmt.Errorf("gocql: column count overflow in newScanValues: metadata predicted %d columns but encountered more", actualSize)
					iter.err = err
					return nil, err
				}
				val, err := elem.NewWithError()
				if err != nil {
					iter.err = err
					return nil, err
				}
				values[idx] = val
				idx++
			}
		}
	}

	if idx != actualSize {
		err := fmt.Errorf("gocql: column count mismatch in newScanValues: metadata predicted %d columns but got %d", actualSize, idx)
		iter.err = err
		return nil, err
	}

	return values, nil
}

// TODO(zariel): is it worth exporting this?
func (iter *Iter) rowMap() (map[string]any, error) {
	if iter.err != nil {
		return nil, iter.err
	}

	rowData, err := iter.RowData()
	if err != nil {
		return nil, err
	}
	iter.Scan(rowData.Values...)
	m := make(map[string]any, len(rowData.Columns))
	rowData.rowMap(m)
	return m, nil
}

// SliceMap is a helper function to make the API easier to use.
// It consumes the remaining rows, closes the iterator, and returns the data
// in the form of []map[string]any.
func (iter *Iter) SliceMap() ([]map[string]any, error) {
	defer iter.Close()

	if iter.err != nil {
		return nil, iter.err
	}

	// Not checking for the error because we just did
	rowData, err := iter.RowData()
	if err != nil {
		return nil, err
	}
	dataToReturn := make([]map[string]any, 0)
	for iter.Scan(rowData.Values...) {
		m := make(map[string]any, len(rowData.Columns))
		rowData.rowMap(m)
		dataToReturn = append(dataToReturn, m)
	}
	if iter.err != nil {
		return nil, iter.err
	}
	return dataToReturn, nil
}

// MapScan takes a map[string]any and populates it with a row
// that is returned from cassandra.
//
// Each call to MapScan() must be called with a new map object.
// During the call to MapScan() any pointers in the existing map
// are replaced with non pointer types before the call returns
//
//	iter := session.Query(`SELECT * FROM mytable`).Iter()
//	for {
//		// New map each iteration
//		row := make(map[string]any)
//		if !iter.MapScan(row) {
//			break
//		}
//		// Do things with row
//		if fullname, ok := row["fullname"]; ok {
//			fmt.Printf("Full Name: %s\n", fullname)
//		}
//	}
//	if err := iter.Close(); err != nil {
//		return err
//	}
//
// You can also pass pointers in the map before each call
//
//	var fullName FullName // Implements gocql.Unmarshaler and gocql.Marshaler interfaces
//	var address net.IP
//	var age int
//	iter := session.Query(`SELECT * FROM scan_map_table`).Iter()
//	for {
//		// New map each iteration
//		row := map[string]any{
//			"fullname": &fullName,
//			"age":      &age,
//			"address":  &address,
//		}
//		if !iter.MapScan(row) {
//			break
//		}
//		fmt.Printf("First: %s Age: %d Address: %q\n", fullName.FirstName, age, address)
//	}
//	if err := iter.Close(); err != nil {
//		return err
//	}
func (iter *Iter) MapScan(m map[string]any) bool {
	if iter.err != nil {
		return false
	}

	rowData, err := iter.RowData()
	if err != nil {
		return false
	}

	for i, col := range rowData.Columns {
		if dest, ok := m[col]; ok {
			rowData.Values[i] = dest
		}
	}

	if iter.Scan(rowData.Values...) {
		rowData.rowMap(m)
		return true
	}
	return false
}

func copyBytes(p []byte) []byte {
	b := make([]byte, len(p))
	copy(b, p)
	return b
}
