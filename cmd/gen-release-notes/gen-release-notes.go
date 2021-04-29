// Copyright (C) 2021 ScyllaDB

package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"time"

	cmd "github.com/scylladb/scylla-operator/pkg/cmd/releasenotes"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rand.Seed(time.Now().UTC().UnixNano())

	klog.InitFlags(flag.CommandLine)
	err := flag.Set("logtostderr", "true")
	if err != nil {
		panic(err)
	}
	defer klog.Flush()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
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
