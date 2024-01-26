// Copyright (C) 2017 ScyllaDB

package uuid

type Value UUID

func (v *Value) String() string {
	if v.Value() == Nil {
		return ""
	}
	return UUID(*v).String()
}

func (v *Value) Set(s string) error {
	return (*UUID)(v).UnmarshalText([]byte(s))
}

func (v *Value) Type() string {
	return "string"
}

func (v *Value) Value() UUID {
	return UUID(*v)
}
