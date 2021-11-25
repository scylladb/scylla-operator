// Copyright (C) 2021 ScyllaDB

package cpuset

import (
	"fmt"
	"math/big"
	"math/bits"
	"strconv"
	"strings"
)

func ParseMaskFormat(words []uint32) CPUSet {
	l := uint(0)
	var set []int
	for i := len(words) - 1; i >= 0; i-- {
		word := words[i]
		for b := l; word != 0; {
			n := uint(bits.TrailingZeros32(word))
			set = append(set, int(b+n))
			word >>= n + 1
			b += n + 1
		}
		l += 32
	}

	return NewCPUSet(set...)
}

func (cs CPUSet) FormatMask() string {
	bitset := new(big.Int)
	for elem := range cs.elems {
		bitset.SetBit(bitset, elem, 1)
	}

	one := big.NewInt(1)
	low32 := new(big.Int).Lsh(one, 32)
	low32.Sub(low32, one)

	mask := ""
	first := true
	zero := new(big.Int)
	for first || bitset.Cmp(zero) != 0 {
		word := new(big.Int).And(bitset, low32)
		mask = fmt.Sprintf(",0x%08x%s", word, mask)
		bitset.Rsh(bitset, 32)
		first = false
	}

	return fmt.Sprintf("%s", mask[1:])
}

func (cs CPUSet) Mask() ([]uint32, error) {
	f := strings.Split(cs.FormatMask(), ",")
	mask := make([]uint32, len(f))
	for i := range f {
		n, err := strconv.ParseUint(strings.TrimPrefix(f[i], "0x"), 16, 32)
		if err != nil {
			return nil, err
		}
		mask[i] = uint32(n)
	}

	return mask, nil
}
