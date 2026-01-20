package duration

type Duration struct {
	Months      int32
	Days        int32
	Nanoseconds int64
}

func (d Duration) Valid() bool {
	return validDuration(d.Months, d.Days, d.Nanoseconds)
}

func validDuration(m, d int32, n int64) bool {
	if m >= 0 && d >= 0 && n >= 0 {
		return true
	}
	if m <= 0 && d <= 0 && n <= 0 {
		return true
	}
	return false
}
