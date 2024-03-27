// Copyright (C) 2021 ScyllaDB

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	cmd "github.com/scylladb/scylla-operator/pkg/cmd/releasenotes"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	klog.InitFlags(flag.CommandLine)
	err := flag.Set("logtostderr", "true")
	if err != nil {
		panic(err)
	}
	defer klog.Flush()

	streams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	cmd := cmd.NewGenGitReleaseNotesCommand(ctx, streams)
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
