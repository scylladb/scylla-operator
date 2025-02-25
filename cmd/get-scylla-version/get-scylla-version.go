// Copyright (C) 2025 ScyllaDB

package main

import (
	"flag"
	"fmt"
	"os"

	scyllasemver "github.com/scylladb/scylla-operator/pkg/semver"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(flag.CommandLine)
	err := flag.Set("logtostderr", "true")
	if err != nil {
		panic(err)
	}
	defer klog.Flush()

	imageName := flag.String("image", "", "Image name to check")
	version := flag.String("version", "", "Version to check")

	flag.Parse()

	if *imageName == "" || *version == "" {
		klog.Fatalf("Usage: %s --image <imageName> --version <version>", os.Args[0])
	}

	semVersion, fullVersion, err := scyllasemver.GetImageVersionAndDigest(*imageName, *version)
	if err != nil {
		klog.Fatalf("Error getting image version: %v", err)
	}

	fmt.Printf("%s %s\n", semVersion, fullVersion)
}
