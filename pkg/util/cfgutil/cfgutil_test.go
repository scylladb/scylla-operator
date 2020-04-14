// Copyright (C) 2017 ScyllaDB

package cfgutil

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
)

func TestParseYAML(t *testing.T) {
	type result struct {
		AuthToken  string `yaml:"auth_token"`
		Prometheus string
		CPU        int
		Logger     map[string]string
	}

	table := []struct {
		Name          string
		Input         []string
		Golden        result
		ErrorExpected bool
	}{
		{
			Name:  "basic",
			Input: []string{"./testdata/base.yaml", "./testdata/first.yaml"},
			Golden: result{
				AuthToken:  "token",
				Prometheus: ":56090",
				CPU:        -2,
				Logger: map[string]string{
					"mode": "stdout",
				},
			},
		},
		{
			Name:  "missing override file",
			Input: []string{"./testdata/base.yaml", "./testdata/missing.yaml"},
			Golden: result{
				AuthToken:  "",
				Prometheus: ":56090",
				CPU:        -1,
				Logger: map[string]string{
					"mode": "stderr",
				},
			},
		},
		{
			Name:  "one missing override file",
			Input: []string{"./testdata/base.yaml", "./testdata/missing.yaml", "./testdata/first.yaml"},
			Golden: result{
				AuthToken:  "token",
				Prometheus: ":56090",
				CPU:        -2,
				Logger: map[string]string{
					"mode": "stdout",
				},
			},
		},
		{
			Name:  "invalid type",
			Input: []string{"./testdata/base.yaml", "./testdata/invalid_type.yaml"},
			Golden: result{
				AuthToken:  "token",
				Prometheus: ":56090",
				CPU:        -2,
				Logger: map[string]string{
					"mode": "stdout",
				},
			},
			ErrorExpected: true,
		},
		{
			Name:  "missing nested field indent",
			Input: []string{"./testdata/base.yaml", "./testdata/missing_nested_field_indent.yaml"},
			Golden: result{
				AuthToken:  "token",
				Prometheus: ":56090",
				CPU:        -2,
				Logger: map[string]string{
					"mode": "stdout",
				},
			},
			ErrorExpected: true,
		},
	}
	for _, test := range table {
		t.Run(test.Name, func(t *testing.T) {
			got := &result{}
			if err := ParseYAML(got, test.Input...); err != nil {
				if test.ErrorExpected {
					if _, ok := err.(*yaml.TypeError); ok {
						return
					}
				}
				t.Error(err)
			}
			if diff := cmp.Diff(*got, test.Golden); diff != "" {
				t.Errorf("%s", diff)
			}
		})
	}
}
