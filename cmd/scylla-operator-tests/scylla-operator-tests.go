// Copyright (C) 2021 ScyllaDB

package main

import (
	"flag"
	"fmt"
	"os"

	cmd "github.com/scylladb/scylla-operator/pkg/cmd/tests"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(flag.CommandLine)
	err := flag.Set("logtostderr", "true")
	if err != nil {
		panic(err)
	}
	defer klog.Flush()

	command := cmd.NewTestsCommand(genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	})
	err = command.Execute()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
