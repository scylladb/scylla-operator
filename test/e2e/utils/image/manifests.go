// Copyright (c) 2022 ScyllaDB.

package image

import "fmt"

// RegistryList holds public and private image registries
type RegistryList struct {
	DockerLibraryRegistry string `json:"dockerLibraryRegistry"`
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
	}
}

const (
	// None is used as unset image
	None = iota

	// BusyBox image
	BusyBox
)

func initImageConfigs(list RegistryList) map[int]Config {
	configs := map[int]Config{}
	configs[BusyBox] = Config{list.DockerLibraryRegistry, "busybox", "1.35"}

	return configs
}

// GetE2EImage returns the fully qualified URI to an image (including version)
func GetE2EImage(image int) string {
	return fmt.Sprintf("%s/%s:%s", imageConfigs[image].registry, imageConfigs[image].name, imageConfigs[image].version)
}
