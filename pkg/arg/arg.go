package arg

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/utils/pointer"
)

func IntFromBool(v bool) int {
	if v {
		return 1
	}
	return 0
}

type Arg struct {
	Flag  string
	Value *string
}

func (a *Arg) String() string {
	if a == nil {
		return ""
	}

	if a.Value == nil {
		return a.Flag
	}

	return fmt.Sprintf("%s=%s", a.Flag, *a.Value)
}

type Args []*Arg

func NewArgs() Args {
	return Args{}
}

func (a *Args) AddArg(flag string) {
	*a = append(*a, &Arg{
		Flag: flag,
	})
}

func (a *Args) AddArgWithIntValue(flag string, value int) {
	*a = append(*a, &Arg{
		Flag:  flag,
		Value: pointer.String(strconv.Itoa(value)),
	})
}

func (a *Args) AddArgWithStringValue(flag string, value string) {
	*a = append(*a, &Arg{
		Flag:  flag,
		Value: &value,
	})
}

/*
// Find finds the last matching flag.
func (a *Args) Find(flag string) (*Arg, error) {
	var res *Arg
	for _, arg := range *a {
		if arg.flag == flag {
			res = &arg
		}
	}

	if res == nil {
		return nil, fmt.Errorf("flag %q not found", flag)
	}

	return res, nil
}

// FindValue finds the last matching flag value.
func (a *Args) FindValue(flag string) (string, error) {
	arg, err := a.Find(flag)
	if err != nil {
		return "", err
	}
	if arg.value == nil {
		return "", fmt.Errorf("flag %q doesn't have a value set", flag)
	}

	return *arg.value, nil
}
*/

func (a *Args) ArgStrings() []string {
	res := make([]string, 0, len(*a))
	for _, arg := range *a {
		if arg == nil {
			continue
		}

		res = append(res, arg.String())
	}

	return res
}

func (a *Args) String() string {
	return strings.Join(a.ArgStrings(), " ")
}
