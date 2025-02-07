package duration

type Duration struct {
	Months      int32
	Days        int32
	Nanoseconds int64
}

func (d Duration) Valid() bool {
	if d.Months >= 0 && d.Days >= 0 && d.Nanoseconds >= 0 {
		return true
	}
	if d.Months <= 0 && d.Days <= 0 && d.Nanoseconds <= 0 {
		return true
	}
	return false
}
