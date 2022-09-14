// Copyright (C) 2021 ScyllaDB

//go:build tools

// Force go mod to download and vendor code that isn't depended upon.
package dependencymagnet

import (
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
	_ "k8s.io/code-generator"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
