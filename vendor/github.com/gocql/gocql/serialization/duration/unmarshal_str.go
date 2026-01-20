package duration

func decString(m, d int32, n int64) string {
	var neg bool
	var tmp uint64
	out := new([50]byte) // max string "-178956970y8mo306783378w2d2562047h47m16.854775808s"
	pos := 49
	if m < 0 || d < 0 || n < 0 {
		neg = true
	}
	if n != 0 {
		tmp = uint64(n)
		if neg {
			tmp = -tmp
		}
		pos = printNanoseconds(out, tmp, pos)
	}
	if d != 0 {
		tmp = uint64(d)
		if neg {
			tmp = -tmp
		}
		pos = printDays(out, tmp, pos)
	}
	if m != 0 {
		tmp = uint64(m)
		if neg {
			tmp = -tmp
		}
		pos = printMonths(out, tmp, pos)
	}
	if pos == 49 {
		return zeroDuration
	}
	if neg {
		out[pos] = '-'
		pos--
	}
	return string(out[pos+1:])
}

func printMonths(out *[50]byte, m uint64, pos int) int {
	months := m % 12
	if months == 0 {
		m--
		months = 12
	}
	out[pos] = 'o'
	pos--
	out[pos] = 'm'
	pos--
	pos = printInt(out, months, pos)
	if m > 12 { // print years
		out[pos] = 'y'
		pos--
		pos = printInt(out, m/12, pos)
	}
	return pos
}

func printDays(out *[50]byte, d uint64, pos int) int {
	days := d % 7
	if days == 0 {
		d--
		days = 7
	}
	out[pos] = 'd' // print days
	pos--
	out[pos] = byte(days) + '0'
	pos--
	if d > 7 { // print weeks
		out[pos] = 'w'
		pos--
		pos = printInt(out, d/7, pos)
	}
	return pos
}

func printNanoseconds(out *[50]byte, n uint64, pos int) int {
	if n < second {
		// Special case: if nanoseconds is smaller than a second,
		// use smaller units, like 1.2ms
		dotPos := 0
		out[pos] = 's'
		pos--
		switch {
		case n < microsecond: // case for nanoseconds
			out[pos] = 'n'
			pos--
		case n < millisecond: // case for microseconds
			copy(out[pos-1:], "µ") // U+00B5 'µ' micro sign == 0xC2 0xB5
			pos -= 2
			if n%microsecond == 0 {
				n /= microsecond
			} else {
				dotPos = 3
			}
		default: // case for milliseconds
			out[pos] = 'm'
			pos--
			if n%millisecond == 0 {
				n /= millisecond
			} else {
				dotPos = 6
			}
		}
		if dotPos == 0 {
			return printInt(out, n, pos)
		}
		return printIntFrac(out, n, dotPos, pos)
	}
	if s := n % 60000000000; s != 0 { // case for seconds
		out[pos] = 's'
		pos--
		pos = printIntFrac(out, s, 9, pos)
	}
	if n >= 60000000000 {
		n /= 60000000000           // n is now integer minutes
		if mn := n % 60; mn != 0 { // case for minutes
			out[pos] = 'm'
			pos--
			pos = printInt(out, mn, pos)
		}
		if n >= 60 {
			out[pos] = 'h'
			pos--
			pos = printInt(out, n/60, pos) // n is now integer hours
		}
	}
	return pos
}

func printIntFrac(out *[50]byte, n uint64, dotPos, pos int) int {
	start := false
	for i := 0; n > 0; i++ {
		digit := n % 10
		if start && i == dotPos {
			out[pos] = '.'
			pos--
		}
		start = start || digit != 0 || i == dotPos
		if start {
			out[pos] = byte(digit) + '0'
			pos--
		}
		n /= 10
	}
	return pos
}

func printInt(out *[50]byte, n uint64, pos int) int {
	switch {
	case n >= 100:
		for n > 0 {
			out[pos] = byte(n%10) + '0'
			n /= 10
			pos--
		}
	case n >= 10:
		out[pos] = byte(n%10) + '0'
		pos--
		out[pos] = byte(n/10) + '0'
		pos--
	default:
		out[pos] = byte(n) + '0'
		pos--
	}
	return pos
}
