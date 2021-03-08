// Copyright (C) 2021 ScyllaDB

// +build tools

// Force go mod to download and vendor code that isn't depended upon.
package dependencymagnet

import (
	_ "k8s.io/code-generator"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
