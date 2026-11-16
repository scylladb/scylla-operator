package configassests

import (
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	ScyllaDBImageRepository             = "docker.io/scylladb/scylla"
	ScyllaDBManagerImageRepository      = "docker.io/scylladb/scylla-manager"
	ScyllaDBManagerAgentImageRepository = "docker.io/scylladb/scylla-manager-agent"
	LocalCSIDriverImageRepository       = "docker.io/scylladb/local-csi-driver"
)

// SupportedPlatforms are the platforms that every image we deploy has to be pullable on.
var SupportedPlatforms = []imgspecv1.Platform{
	{
		OS:           "linux",
		Architecture: "amd64",
	},
	{
		OS:           "linux",
		Architecture: "arm64",
	},
}
