package internalapi

type SidecarRuntimeConfig struct {
	// containerID hold the ID of the scylla container this information is valid for.
	// E.g. on restarts, the container gets a new ID.
	ContainerID string `json:"containerID"`

	// matchingNodeConfigs is a list of NodeConfigs that affect this pod.
	MatchingNodeConfigs []string `json:"matchingNodeConfigs"`

	// blockingNodeConfigs is a list of NodeConfigs this pod is waiting on.
	BlockingNodeConfigs []string `json:"blockingNodeConfigs"`
}
