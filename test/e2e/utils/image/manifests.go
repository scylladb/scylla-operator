// Copyright (c) 2023 ScyllaDB.

package image

import "fmt"

// RegistryList holds public and private image registries
type RegistryList struct {
	DockerLibraryRegistry string `json:"dockerLibraryRegistry"`
	QuayScyllaDB          string `json:"quayScyllaDB"`
}

// Config holds an images registry, name, and version
type Config struct {
	registry string
	name     string
	version  string
}

var (
	registry = initRegistry()

	imageConfigs = initImageConfigs(registry)
)

func initRegistry() RegistryList {
	return RegistryList{
		DockerLibraryRegistry: "docker.io/library",
		QuayScyllaDB:          "quay.io/scylladb",
	}
}

const (
	// None is used as unset image
	None = iota

	// BusyBox image
	BusyBox

	// OperatorNodeSetup image
	OperatorNodeSetup
)

func initImageConfigs(list RegistryList) map[int]Config {
	configs := map[int]Config{}
	configs[BusyBox] = Config{registry: list.DockerLibraryRegistry, name: "busybox", version: "1.35"}
	configs[OperatorNodeSetup] = Config{registry: list.QuayScyllaDB, name: "scylla-operator-images", version: "node-setup-v0.0.1@sha256:a25e77c769605bbb1f9c8b0b29723bb6b1d9990982d43c908779a0717dc10cb4"}

	return configs
}

// GetE2EImage returns the fully qualified URI to an image (including version)
func GetE2EImage(image int) string {
	return fmt.Sprintf("%s/%s:%s", imageConfigs[image].registry, imageConfigs[image].name, imageConfigs[image].version)
}
