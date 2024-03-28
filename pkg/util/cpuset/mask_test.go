// Copyright (C) 2021 ScyllaDB

package cpuset

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
)

func TestMask(t *testing.T) {
	ts := []struct {
		Mask     []uint32
		Expected string
	}{
		{Mask: []uint32{0x00007fff}, Expected: "0-14"},
		{Mask: []uint32{0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff}, Expected: "0-127"},
		{Mask: []uint32{0x00000001}, Expected: "0"},
		{Mask: []uint32{0x40000000, 0x00000000, 0x00000000}, Expected: "94"},
		{Mask: []uint32{0x00000001, 0x00000000, 0x00000000}, Expected: "64"},
		{Mask: []uint32{0x000000ff, 0x00000000}, Expected: "32-39"},
		{Mask: []uint32{0x000e3862}, Expected: "1,5-6,11-13,17-19"},
		{Mask: []uint32{0x00000001, 0x00000001, 0x00010117}, Expected: "0-2,4,8,16,32,64"},
	}
	for _, test := range ts {
		t.Run(test.Expected, func(t *testing.T) {
			mask := ParseMaskFormat(test.Mask).String()
			if mask != test.Expected {
				t.Errorf("Expected %v, got %v", test.Expected, mask)
			}
			cpuset, err := Parse(test.Expected)
			if err != nil {
				t.Errorf("Expected non-nil error, got %s", err)
			}

			m, err := cpuset.Mask()
			if err != nil {
				t.Errorf("Expected non-nil error, got %s", err)
			}
			if !equality.Semantic.DeepEqual(m, test.Mask) {
				t.Errorf("Expected %v mask, got %v", test.Mask, m)
			}
		})
	}
}

func integerRange(a, b int) []int {
	r := make([]int, 0, b-a+1)
	for i := a; i <= b; i++ {
		r = append(r, i)
	}
	return r
}

func TestFormatMask(t *testing.T) {
	ts := []struct {
		CPUs     []int
		Expected string
	}{
		{CPUs: integerRange(0, 14), Expected: "0x00007fff"},
		{CPUs: integerRange(0, 127), Expected: "0xffffffff,0xffffffff,0xffffffff,0xffffffff"},
		{CPUs: integerRange(0, 0), Expected: "0x00000001"},
		{CPUs: integerRange(94, 94), Expected: "0x40000000,0x00000000,0x00000000"},
		{CPUs: integerRange(64, 64), Expected: "0x00000001,0x00000000,0x00000000"},
		{CPUs: integerRange(32, 39), Expected: "0x000000ff,0x00000000"},
		{CPUs: []int{1, 5, 6, 11, 12, 13, 17, 18, 19}, Expected: "0x000e3862"},
		{CPUs: []int{0, 1, 2, 4, 8, 16, 32, 64}, Expected: "0x00000001,0x00000001,0x00010117"},
	}
	for _, test := range ts {
		t.Run(test.Expected, func(t *testing.T) {
			cpuset := NewCPUSet(test.CPUs...)
			mask := cpuset.FormatMask()
			if mask != test.Expected {
				t.Errorf("Expected %v, got %v", test.Expected, mask)
			}
		})
	}

}
