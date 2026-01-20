package duration

import (
	"errors"
	"fmt"
)

const (
	nanosecond  = 1
	microsecond = 1000 * nanosecond
	millisecond = 1000 * microsecond
	second      = 1000 * millisecond
	minute      = 60 * second
	hour        = 60 * minute
	week        = 7
	year        = 12

	microsecondFloat = float64(microsecond)
	millisecondFloat = float64(millisecond)
	secondFloat      = float64(second)
	minuteFloat      = float64(minute)
	hourFloat        = float64(hour)
	weekFloat        = float64(week)
	yearFloat        = float64(year)

	maxNanosecondsNeg = uint64(9223372036854775808)
	maxNanosecondsPos = maxNanosecondsNeg - 1
	maxMonthsDaysNeg  = uint64(2147483648)
	maxMonthsDaysPos  = maxMonthsDaysNeg - 1

	maxMicrosecondsNeg = maxNanosecondsNeg / microsecond
	maxMillisecondsNeg = maxMicrosecondsNeg / 1000
	maxSecondsNeg      = maxMillisecondsNeg / 1000
	maxMinutesNeg      = maxSecondsNeg / 60
	maxHoursNeg        = maxMinutesNeg / 60
	maxWeeksNeg        = maxNanosecondsNeg / 7
	maxYearsNeg        = maxNanosecondsNeg / 12
)

type parseReadState byte

const (
	readInteger parseReadState = iota
	readFraction
	readUnit
	readSkipFraction
)

type parseWriteState byte

const (
	writeNanoseconds parseWriteState = iota
	writeMilliseconds
	writeMicroseconds
	writeSeconds
	writeMinutes
	writeHours
	writeDays
	writeWeeks
	writeMonths
	writeYears
)

func errorOutRange(valName string, integer uint64) error {
	return fmt.Errorf("%s %d out of the range", valName, integer)
}

func errorUnknownChars(chars string) error {
	return fmt.Errorf("unknown charesters \"%s\"", chars)
}

func encString(s string) ([]byte, error) {
	months, days, nanos, neg, err := encStringToUints(s)
	if err != nil {
		return nil, err
	}
	if neg {
		return encVintMonthsDaysNanosNeg(months, days, nanos), nil
	}
	return encVintMonthsDaysNanosPos(months, days, nanos), nil
}

func encStringToUints(s string) (months uint64, days uint64, nanos uint64, neg bool, err error) {
	// Special case: if all that is left is "0", this is zero.
	if s == zeroDuration {
		return 0, 0, 0, false, nil
	}
	// get are sing
	if c := s[0]; c == '-' || c == '+' {
		neg = c == '-'
		s = s[1:]
	}

	var writeState parseWriteState
	readState := readInteger
	scale := float64(1)
	var integer, fraction uint64
	var ok bool
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch readState {
		case readInteger: // Consume [0-9.]* as integer part of value
			switch {
			case c >= '0' && c <= '9':
				integer = integer*10 + uint64(c) - '0'
				if integer > maxNanosecondsNeg {
					return 0, 0, 0, false, errorOutRange("value", integer)
				}
			case c == '.':
				readState = readFraction
			default:
				i--
				readState = readUnit
			}
		case readFraction: // Consume [0-9]* as fraction part of value
			if c >= '0' && c <= '9' {
				scale *= 10
				fraction = fraction*10 + uint64(c) - '0'
				if fraction > maxNanosecondsNeg {
					readState = readSkipFraction
					fraction /= 10
				}
			} else {
				i--
				readState = readUnit
			}
		case readUnit: // Consume unit part of the string, adding the integer and fraction parts of the value to the output values.
			// Supported units:
			// "ns"	nanosecond,
			// "us"	microsecond,
			// "µs"	microsecond U+00B5 = micro symbol,
			// "μs" microsecond U+03BC = Greek letter mu
			// "ms"	millisecond,
			// "s"	second,
			// "m"	minute,
			// "h"	hour,
			// "d"	day,
			// "w"	week,
			// "mo"	month,
			// "y"	year,

			switch c {
			case 'n': // "ns" nanosecond
				if i+1 == len(s) || s[i+1] != 's' {
					return 0, 0, 0, false, errorUnknownChars(s[i:])
				}
				i++
				writeState = writeNanoseconds
			case 'u': // "us" microsecond
				if i+1 == len(s) || s[i+1] != 's' {
					return 0, 0, 0, false, errorUnknownChars(s[i:])
				}
				i++
				writeState = writeMicroseconds
			case 194: // "µs" microsecond U+00B5 = micro symbol
				if i+2 >= len(s) || s[i+1] != 181 || s[i+2] != 's' {
					return 0, 0, 0, false, errorUnknownChars(s[i:])
				}
				i++
				writeState = writeMicroseconds
			case 206: // "μs" microsecond U+03BC = Greek letter mu
				if i+2 >= len(s) || s[i+1] != 188 || s[i+2] != 's' {
					return 0, 0, 0, false, errorUnknownChars(s[i:])
				}
				i++
				writeState = writeMicroseconds
			case 'm': // "ms" millisecond,"mo" month,"m" minute,
				if i+1 == len(s) { // "m" minute
					writeState = writeMinutes
				} else {
					switch s[i+1] { // "ms" millisecond,"mo" month,"m" minute,
					case 's': // "ms" millisecond
						i++
						writeState = writeMilliseconds
					case 'o': // "mo" month
						i++
						writeState = writeMonths
					case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.': // "m" minute
						writeState = writeMinutes
					default:
						return 0, 0, 0, false, errorUnknownChars(s[i:])
					}
				}
			case 's': // "s" second
				writeState = writeSeconds
			case 'h': // "h" hour
				writeState = writeHours
			case 'd': // "d" day
				writeState = writeDays
			case 'w': // "w" week
				writeState = writeWeeks
			case 'y': // "y" year
				writeState = writeYears
			default: // unsupported characters
				return 0, 0, 0, false, errorUnknownChars(s[i:])
			}

			switch writeState {
			case writeNanoseconds:
				if nanos, ok = addNanoseconds(nanos, integer, fraction, scale); !ok {
					return 0, 0, 0, false, errorOutRange("nanoseconds", nanos)
				}
			case writeMicroseconds:
				if nanos, ok = addMicroseconds(nanos, integer, fraction, scale); !ok {
					return 0, 0, 0, false, errorOutRange("nanoseconds", nanos)
				}
			case writeMilliseconds:
				if nanos, ok = addMilliseconds(nanos, integer, fraction, scale); !ok {
					return 0, 0, 0, false, errorOutRange("nanoseconds", nanos)
				}
			case writeSeconds:
				if nanos, ok = addSeconds(nanos, integer, fraction, scale); !ok {
					return 0, 0, 0, false, errorOutRange("nanoseconds", nanos)
				}
			case writeMinutes:
				if nanos, ok = addMinutes(nanos, integer, fraction, scale); !ok {
					return 0, 0, 0, false, errorOutRange("nanoseconds", nanos)
				}
			case writeHours:
				if nanos, ok = addHours(nanos, integer, fraction, scale); !ok {
					return 0, 0, 0, false, errorOutRange("nanoseconds", nanos)
				}
			case writeDays:
				if days, ok = addDaysMonths(days, integer, fraction, scale); !ok {
					return 0, 0, 0, false, errorOutRange("days", days)
				}
			case writeWeeks:
				if days, ok = addWeeks(days, integer, fraction, scale); !ok {
					return 0, 0, 0, false, errorOutRange("days", days)
				}
			case writeMonths:
				if months, ok = addDaysMonths(months, integer, fraction, scale); !ok {
					return 0, 0, 0, false, errorOutRange("months", months)
				}
			default: // writeYears
				if months, ok = addYears(months, integer, fraction, scale); !ok {
					return 0, 0, 0, false, errorOutRange("years", months)
				}
			}

			// reset the temporary values, after write
			readState = readInteger
			integer, fraction, scale = 0, 0, 1
		default: // Consume [0-9]* in case with overflow of the fraction part of value. Just skip digits.
			if c < '0' || c > '9' {
				i--
				readState = readUnit
			}
		}
	}
	if integer != 0 || fraction != 0 || scale != 1 { // if the temporary values are not reset, it means that the reading is not fully completed.
		return 0, 0, 0, false, errors.New("unsupported format")
	}
	if !neg {
		if months > maxMonthsDaysPos {
			return 0, 0, 0, false, errorOutRange("months", months)
		}
		if days > maxMonthsDaysPos {
			return 0, 0, 0, false, errorOutRange("days", days)
		}
		if nanos > maxNanosecondsPos {
			return 0, 0, 0, false, errorOutRange("nanoseconds", days)
		}
	}
	return
}

func addNanoseconds(in, add, fraction uint64, scale float64) (uint64, bool) {
	if fraction > 0 {
		add += uint64(float64(fraction) * (float64(1) / scale))
		if add > maxNanosecondsNeg {
			return add, false
		}
	}
	in += add
	if in > maxNanosecondsNeg {
		return in, false
	}
	return in, true
}

func addMicroseconds(in, add, fraction uint64, scale float64) (uint64, bool) {
	if add > maxMicrosecondsNeg {
		return add, false
	}
	add *= microsecond
	if fraction > 0 {
		add += uint64(float64(fraction) * (microsecondFloat / scale))
		if add > maxNanosecondsNeg {
			return add, false
		}
	}
	in += add
	if in > maxNanosecondsNeg {
		return in, false
	}
	return in, true
}

func addMilliseconds(in, add, fraction uint64, scale float64) (uint64, bool) {
	if add > maxMillisecondsNeg {
		return add, false
	}
	add *= millisecond
	if fraction > 0 {
		add += uint64(float64(fraction) * (millisecondFloat / scale))
		if add > maxNanosecondsNeg {
			return add, false
		}
	}
	in += add
	if in > maxNanosecondsNeg {
		return in, false
	}
	return in, true
}

func addSeconds(in, add, fraction uint64, scale float64) (uint64, bool) {
	if add > maxSecondsNeg {
		return add, false
	}
	add *= second
	if fraction > 0 {
		add += uint64(float64(fraction) * (secondFloat / scale))
		if add > maxNanosecondsNeg {
			return add, false
		}
	}
	in += add
	if in > maxNanosecondsNeg {
		return in, false
	}
	return in, true
}

func addMinutes(in, add, fraction uint64, scale float64) (uint64, bool) {
	if add > maxMinutesNeg {
		return add, false
	}
	add *= minute
	if fraction > 0 {
		add += uint64(float64(fraction) * (minuteFloat / scale))
		if add > maxNanosecondsNeg {
			return add, false
		}
	}
	in += add
	if in > maxNanosecondsNeg {
		return in, false
	}
	return in, true
}

func addHours(in, add, fraction uint64, scale float64) (uint64, bool) {
	if add > maxHoursNeg {
		return add, false
	}
	add *= hour
	if fraction > 0 {
		add += uint64(float64(fraction) * (hourFloat / scale))
		if add > maxNanosecondsNeg {
			return add, false
		}
	}
	in += add
	if in > maxNanosecondsNeg {
		return in, false
	}
	return in, true
}

func addDaysMonths(in, add, fraction uint64, scale float64) (uint64, bool) {
	if fraction > 0 {
		add += uint64(float64(fraction) * (float64(1) / scale))
		if add > maxMonthsDaysNeg {
			return add, false
		}
	}
	in += add
	if in > maxMonthsDaysNeg {
		return in, false
	}
	return in, true
}

func addWeeks(in, add, fraction uint64, scale float64) (uint64, bool) {
	if add > maxWeeksNeg {
		return add, false
	}
	add *= week
	if fraction > 0 {
		add += uint64(float64(fraction) * (weekFloat / scale))
		if add > maxMonthsDaysNeg {
			return add, false
		}
	}
	in += add
	if in > maxMonthsDaysNeg {
		return in, false
	}
	return in, true
}

func addYears(in, add, fraction uint64, scale float64) (uint64, bool) {
	if add > maxYearsNeg {
		return add, false
	}
	add *= year
	if fraction > 0 {
		add += uint64(float64(fraction) * (yearFloat / scale))
		if add > maxMonthsDaysNeg {
			return add, false
		}
	}
	in += add
	if in > maxMonthsDaysNeg {
		return in, false
	}
	return in, true
}
