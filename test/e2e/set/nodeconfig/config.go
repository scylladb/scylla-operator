// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"time"

	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	testTimeout              = 15 * time.Minute
	nodeConfigRolloutTimeout = 5 * time.Minute
	apiCallTimeout           = 5 * time.Second
)

var (
	nodeConfigResourceInfo = collect.ResourceInfo{
		Resource: schema.GroupVersionResource{
			Group:    "scylla.scylladb.com",
			Version:  "v1alpha1",
			Resource: "nodeconfigs",
		},
		Scope: meta.RESTScopeRoot,
	}
	resourceQuotaResourceInfo = collect.ResourceInfo{
		Resource: schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "resourcequotas",
		},
		Scope: meta.RESTScopeNamespace,
	}
)
