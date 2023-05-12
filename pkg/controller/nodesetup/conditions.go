// Copyright (c) 2023 ScyllaDB.

package nodesetup

const (
	raidControllerNodeProgressingConditionFormat = "RaidControllerNode%sProgressing"
	raidControllerNodeDegradedConditionFormat    = "RaidControllerNode%sDegraded"

	filesystemControllerNodeProgressingConditionFormat = "FilesystemControllerNode%sProgressing"
	filesystemControllerNodeDegradedConditionFormat    = "FilesystemControllerNode%sDegraded"

	mountControllerNodeProgressingConditionFormat = "MountControllerNode%sProgressing"
	mountControllerNodeDegradedConditionFormat    = "MountControllerNode%sDegraded"
)
