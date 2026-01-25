// Copyright (c) 2026 ScyllaDB.

package helpers_test

import (
	"reflect"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/pointer"
)

func TestParseScyllaArguments(t *testing.T) {
	ts := []struct {
		Name         string
		Args         string
		ExpectedArgs map[string]*string
	}{
		{
			Name:         "empty",
			Args:         "",
			ExpectedArgs: map[string]*string{},
		},
		{
			Name:         "single argument with equals",
			Args:         "--rpc-address=10.0.0.1",
			ExpectedArgs: map[string]*string{"rpc-address": pointer.Ptr("10.0.0.1")},
		},
		{
			Name:         "single argument with space",
			Args:         "--listen-address 192.168.1.100",
			ExpectedArgs: map[string]*string{"listen-address": pointer.Ptr("192.168.1.100")},
		},
		{
			Name:         "IPv6 address",
			Args:         "--rpc-address=2001:db8::1",
			ExpectedArgs: map[string]*string{"rpc-address": pointer.Ptr("2001:db8::1")},
		},
		{
			Name:         "wildcard addresses",
			Args:         "--rpc-address 0.0.0.0",
			ExpectedArgs: map[string]*string{"rpc-address": pointer.Ptr("0.0.0.0")},
		},
		{
			Name: "multiple arguments",
			Args: "--rpc-address=10.0.0.1 --listen-address=10.0.0.1",
			ExpectedArgs: map[string]*string{
				"rpc-address":    pointer.Ptr("10.0.0.1"),
				"listen-address": pointer.Ptr("10.0.0.1"),
			},
		},
		{
			Name:         "quoted value",
			Args:         `--workdir="/var/lib/scylla"`,
			ExpectedArgs: map[string]*string{"workdir": pointer.Ptr(`"/var/lib/scylla"`)},
		},
		{
			Name:         "flag without value",
			Args:         "--developer-mode",
			ExpectedArgs: map[string]*string{"developer-mode": nil},
		},
	}

	for _, test := range ts {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			argumentsMap := helpers.ParseScyllaArguments(test.Args)
			if !reflect.DeepEqual(test.ExpectedArgs, argumentsMap) {
				t.Errorf("expected %+v, got %+v", test.ExpectedArgs, argumentsMap)
			}
		})
	}
}
