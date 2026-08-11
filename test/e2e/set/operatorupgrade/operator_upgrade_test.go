// Copyright (C) 2026 ScyllaDB

package operatorupgrade

import (
	"testing"

	o "github.com/onsi/gomega"
	configassets "github.com/scylladb/scylla-operator/assets/config"
)

func TestGetOperatorImageRef(t *testing.T) {
	o.RegisterTestingT(t)

	o.Expect(getOperatorImageRef("1.21.0")).To(o.Equal(configassets.OperatorImageRepository + ":1.21.0"))
	o.Expect(getOperatorImageRef("localhost:5001/scylladb/scylla-operator@sha256:1111111111111111111111111111111111111111111111111111111111111111")).To(o.Equal("localhost:5001/scylladb/scylla-operator@sha256:1111111111111111111111111111111111111111111111111111111111111111"))
}

func TestGetDeployScriptForImageRef(t *testing.T) {
	tt := []struct {
		name                 string
		operatorImageRef     string
		expectedDeployScript string
	}{
		{
			name:                 "released version",
			operatorImageRef:     configassets.OperatorImageRepository + ":1.21.0",
			expectedDeployScript: releaseDeployScript,
		},
		{
			name:                 "release candidate version",
			operatorImageRef:     configassets.OperatorImageRepository + ":1.21.1-rc.0",
			expectedDeployScript: releaseDeployScript,
		},
		{
			name:                 "latest tag follows the master checkout convention",
			operatorImageRef:     configassets.OperatorImageRepository + ":latest",
			expectedDeployScript: masterDeployScript,
		},
		{
			name:                 "digest-only kind local registry ref",
			operatorImageRef:     "localhost:5001/scylladb/scylla-operator@sha256:1111111111111111111111111111111111111111111111111111111111111111",
			expectedDeployScript: masterDeployScript,
		},
		{
			name:                 "bare untagged ref",
			operatorImageRef:     configassets.OperatorImageRepository,
			expectedDeployScript: masterDeployScript,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// getDeployScriptForImageRef asserts internally via the global gomega, which requires a registered handler.
			o.RegisterTestingT(t)

			o.Expect(getDeployScriptForImageRef(tc.operatorImageRef)).To(o.Equal(tc.expectedDeployScript))
		})
	}
}
