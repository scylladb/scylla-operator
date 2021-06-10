// Copyright (C) 2021 ScyllaDB

package resourceapply

import (
	"math/big"
	"testing"
)

type A struct {
	I int
	S string
}

func hashObjectsOrDie(objs ...interface{}) string {
	hash, err := HashObjects(objs...)
	if err != nil {
		panic(err)
	}
	return hash
}

func TestDeepHashObject(t *testing.T) {
	const iterations = 100

	tests := []struct {
		Name    string
		Factory func() interface{}
	}{
		{"int", func() interface{} { return 123 }},
		{"bit int", func() interface{} { return big.NewInt(1<<63 - 1) }},
		{"float", func() interface{} { return 1.23 }},
		{"bool", func() interface{} { return true }},
		{"string", func() interface{} { return "Yumiko" }},
		{"struct", func() interface{} { return A{123, "Hello World"} }},
		{"ptr struct", func() interface{} { return &A{123, "Hello World"} }},
		{"string slice", func() interface{} { return []string{"Come", "Yumiko"} }},
		{"int slice", func() interface{} { return []int{2, 1, 3, 7} }},
		{"struct slice", func() interface{} { return []A{{1, "1"}, {2, "2"}} }},
		{"ptr struct slice", func() interface{} { return []*A{{1, "1"}, {2, "2"}} }},
		{"string string map", func() interface{} { return map[string]string{"Hello": "World"} }},
		{"string int map", func() interface{} { return map[string]int{"Hello": 123, "World": 321} }},
		{"string any map", func() interface{} {
			return map[string]interface{}{"Hello": map[string]interface{}{"Yumiko": &A{1, "1"}}}
		}},
	}
	for i := range tests {
		test := tests[i]
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			f := test.Factory
			hashes := make(map[string]struct{}, 1)
			hashes[hashObjectsOrDie(f())] = struct{}{}

			for i := 0; i < iterations; i++ {
				hash := hashObjectsOrDie(f())
				_, collision := hashes[hash]
				if !collision {
					t.Fatalf("hash of the same object %v is different between two runs: hash1: %q, hash2: %q", f(), hash, hashes[hash])
				}
				hashes[hash] = struct{}{}
			}
		})
	}
}
