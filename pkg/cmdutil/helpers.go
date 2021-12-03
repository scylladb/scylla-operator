// Copyright (C) 2021 ScyllaDB

package cmdutil

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/errors"
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

	return errors.NewAggregate(errs)
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

func GetLoglevel() string {
	f := flag.CommandLine.Lookup("v")
	if f != nil {
		return f.Value.String()
	}

	return ""
}
