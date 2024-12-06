// Copyright (C) 2021 ScyllaDB

package e2e

import (
	_ "github.com/scylladb/scylla-operator/test/e2e/set/nodeconfig"
	_ "github.com/scylladb/scylla-operator/test/e2e/set/remotekubernetescluster"
	_ "github.com/scylladb/scylla-operator/test/e2e/set/scyllacluster"
	_ "github.com/scylladb/scylla-operator/test/e2e/set/scylladbcluster"
	_ "github.com/scylladb/scylla-operator/test/e2e/set/scylladbdatacenter"
	_ "github.com/scylladb/scylla-operator/test/e2e/set/scylladbmonitoring"
	_ "github.com/scylladb/scylla-operator/test/e2e/set/scyllaoperatorconfig"
)
