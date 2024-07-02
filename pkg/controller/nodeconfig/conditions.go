// Copyright (C) 2024 ScyllaDB

package nodeconfig

const (
	namespaceControllerProgressingCondition          = "NamespaceControllerProgressing"
	namespaceControllerDegradedCondition             = "NamespaceControllerDegraded"
	clusterRoleControllerProgressingCondition        = "ClusterRoleControllerProgressing"
	clusterRoleControllerDegradedCondition           = "ClusterRoleControllerDegraded"
	serviceAccountControllerProgressingCondition     = "ServiceAccountControllerProgressing"
	serviceAccountControllerDegradedCondition        = "ServiceAccountControllerDegraded"
	clusterRoleBindingControllerProgressingCondition = "ClusterRoleBindingControllerProgressing"
	clusterRoleBindingControllerDegradedCondition    = "ClusterRoleBindingControllerDegraded"
	daemonSetControllerProgressingCondition          = "DaemonSetControllerProgressing"
	daemonSetControllerDegradedCondition             = "DaemonSetControllerDegraded"
)
