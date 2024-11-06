// Copyright (c) 2023 ScyllaDB.

package nodesetup

const (
	raidControllerNodeSetupProgressingConditionFormat = "RaidControllerNodeSetup%sProgressing"
	raidControllerNodeSetupDegradedConditionFormat    = "RaidControllerNodeSetup%sDegraded"

	filesystemControllerNodeSetupProgressingConditionFormat = "FilesystemControllerNodeSetup%sProgressing"
	filesystemControllerNodeSetupDegradedConditionFormat    = "FilesystemControllerNodeSetup%sDegraded"

	mountControllerNodeSetupProgressingConditionFormat = "MountControllerNodeSetup%sProgressing"
	mountControllerNodeSetupDegradedConditionFormat    = "MountControllerNodeSetup%sDegraded"

	loopDeviceControllerNodeSetupProgressingConditionFormat = "LoopDeviceControllerNodeSetup%sProgressing"
	loopDeviceControllerNodeSetupDegradedConditionFormat    = "LoopDeviceControllerNodeSetup%sDegraded"

	//TODO(rzetelskik): remove deprecated conditions in >=1.16
	deprecatedRaidControllerNodeSetupProgressingConditionFormat = "RaidControllerNode%sProgressing"
	deprecatedRaidControllerNodeSetupDegradedConditionFormat    = "RaidControllerNode%sDegraded"

	deprecatedFilesystemControllerNodeSetupProgressingConditionFormat = "FilesystemControllerNode%sProgressing"
	deprecatedFilesystemControllerNodeSetupDegradedConditionFormat    = "FilesystemControllerNode%sDegraded"

	deprecatedMountControllerNodeSetupProgressingConditionFormat = "MountControllerNode%sProgressing"
	deprecatedMountControllerNodeSetupDegradedConditionFormat    = "MountControllerNode%sDegraded"

	deprecatedLoopDeviceControllerNodeSetupProgressingConditionFormat = "LoopDeviceControllerNode%sProgressing"
	deprecatedLoopDeviceControllerNodeSetupDegradedConditionFormat    = "LoopDeviceControllerNode%sDegraded"
)
