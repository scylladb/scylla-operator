// Copyright (c) 2023 ScyllaDB.

package upgrade

import "github.com/spf13/pflag"

func AddFlags(fs *pflag.FlagSet) {
	addOperatorFlags(fs)
}
