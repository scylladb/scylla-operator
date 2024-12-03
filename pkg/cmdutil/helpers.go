// Copyright (C) 2021 ScyllaDB

package cmdutil

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

const (
	FlagLogLevelKey = "loglevel"
)

func UsageError(cmd *cobra.Command, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s\nSee '%s -h' for help and examples.", msg, cmd.CommandPath())
}

func NormalizeNameForEnvVar(name string) string {
	s := strings.ToUpper(name)
	s = strings.Replace(s, "-", "_", -1)
	return s
}

func ReadFlagsFromEnv(prefix string, cmd *cobra.Command) error {
	var errs []error
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			// flags always take precedence over environment
			return
		}

		// See if there exists matching environment variable
		envVarName := NormalizeNameForEnvVar(prefix + f.Name)
		v, exists := os.LookupEnv(envVarName)
		if !exists {
			return
		}

		err := f.Value.Set(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't parse env var %q with value %q into flag %q: %v", envVarName, v, f.Name, err))
			return
		}

		f.Changed = true

		return
	})

	return utilerrors.NewAggregate(errs)
}

func InstallKlog(cmd *cobra.Command) {
	level := flag.CommandLine.Lookup("v").Value.(*klog.Level)
	levelPtr := (*int32)(level)
	cmd.PersistentFlags().Int32Var(levelPtr, FlagLogLevelKey, *levelPtr, "Set the level of log output (0-10).")
	if cmd.PersistentFlags().Lookup("v") == nil {
		cmd.PersistentFlags().Int32Var(levelPtr, "v", *levelPtr, "Set the level of log output (0-10).")
	}
	cmd.PersistentFlags().Lookup("v").Hidden = true

	// Enable directory prefix.
	err := flag.CommandLine.Lookup("add_dir_header").Value.Set("true")
	if err != nil {
		panic(err)
	}
}

func getLoglevelOrDefault() (int, error) {
	f := flag.CommandLine.Lookup("v")
	if f == nil {
		return 0, errors.New(`can't lookup klog "v" flag`)
	}

	s := f.Value.String()
	if len(s) == 0 {
		return 0, nil
	}

	loglevel, err := strconv.Atoi(s)
	if err != nil {
		panic(fmt.Errorf("can't parse log level %q: %q", s, err))
	}

	return loglevel, nil
}

func GetLoglevelOrDefaultOrDie() int {
	return helpers.Must(getLoglevelOrDefault())
}
