// Copyright (C) 2017 ScyllaDB

package cfgutil

import (
	"os"

	"go.uber.org/config"
)

// ParseYAML attempts to load and parse the files given by files and store
// the contents of the files in the struct given by target.
// It will overwrite any conflicting keys by the keys in the subsequent files.
// Missing files will not cause an error but will just be skipped.
func ParseYAML(target interface{}, files ...string) error {
	var opts []config.YAMLOption
	for _, f := range files {
		if fileExists(f) {
			opts = append(opts, config.File(f))
		}
	}
	cfg, err := config.NewYAML(opts...)
	if err != nil {
		return err
	}
	if err := cfg.Get(config.Root).Populate(target); err != nil {
		return err
	}
	return nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
