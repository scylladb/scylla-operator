# Performance improvements using perftune.py

## Summary

Most of the optimizations are implemented in a perftune.py script maintained by the Scylla Core team.
Operator will be responsible for setting up correct permissions and mounts required for optimizations running
in the container to land on the host machine.

## Motivation

### Goals

* K8s Nodes running Scylla Pods are optimized using perftune.py
* All optimizations implemented in perftune.py land on host machine

### Non-Goals

* Revert optimizations when DaemonSet is uninstalled

## Proposal

We are going to use a privileged DaemonSet responsible for running the perftune.py script on each Node having Scylla Pod.

Because we are tuning cluster wide resources (Nodes), we cannot have a switch inside ScyllaCluster CRD.
That’s why we will expose an additional non-namespaced CRD consisting of placement requirements and tuning options.
Later this CRD can be extended with other setup and configuration logic like disk formatting and partitioning.

### User Stories

#### Story 1

As a user I have several multi-purpose K8s nodes, and a couple of them are dedicated just for Scylla.
I would like to tune just those dedicated to Scylla. Others shouldn't be touched because optimizations may interfere with other applications.

User is able to create multiple ScyllaNodeConfigs to distinguish multiple types of nodes and different setups.

#### Story 2

As a user I have multiple K8s environments. Some of them have different storage then others, and they have different setups. 
I would like to disable optimizations on nodes which doesn't have recommended storage and configuration.

User is able to completely disable optimizations, and to select nodes which should/shouldn't be tuned.

### Risks and Mitigations

Perftune scripts require access to physical devices, host networking, PID namespace and write permission to several paths from
the host machine (/etc, /sys/fs, …).
Limiting the elevated privileges to a namespace managed by cluster administrator will avoid the need to grant these 
privileges to regular users creating ScyllaClusters and retain multitenancy. 
These elevated privileges are equivalent to being system:admin, the most privileged user in the system.
It will also limit the surface area if the attacker gains control over Scylla or the namespace.

## Design Details

#### User facing API

```go

type ScyllaOperatorConfigSpec struct {
	// scyllaUtilsImage is an image used by utility containers requiring Scylla scripts. 
	ScyllaUtilsImage string `json:"scyllaUtilsImage"`
}


// +kubebuilder:resource:path=scyllaoperatorconfigs,scope=Cluster
// +genclient:nonNamespaced
type ScyllaOperatorConfigStatus struct {
}

type ScyllaOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScyllaOperatorConfigSpec   `json:"spec,omitempty"`
	Status ScyllaOperatorConfigStatus `json:"status,omitempty"`
}

type ConditionType string

const (
	// ScyllaNodeConfigAvailable means the ScyllaNodeConfig is available, ie. replicas required are up and running.
	ScyllaNodeConfigAvailable ConditionType = "Available"

	// ScyllaNodeConfigScalingReason is added when ScyllaNodeConfig hasn't reached desired number of replicas.
	ScyllaNodeConfigScalingReason = "Scaling"

	// ScyllaNodeConfigReplicasReadyReason is added when all required ScyllaNodeConfig replicas are ready.
	ScyllaNodeConfigReplicasReadyReason = "ReplicasReady"
)

type Condition struct {
	// type of condition.
	Type ConditionType `json:"type"`

	// status of the condition, one of True, False, or Unknown.
	Status corev1.ConditionStatus `json:"status"`

	// lastTransitionTime is last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// reason for the condition's last transition.
	Reason string `json:"reason"`

	// message is a human-readable message indicating details about the transition.
	Message string `json:"message"`
}

type ReplicaStatus struct {
	// desired is the number of Pods that we expect to have.
	Desired int32 `json:"desired"`

	// actual is the number of Pods created.
	Actual int32 `json:"actual"`
    
	// ready is the number of Pods created that have a Ready Condition.
	Ready int32 `json:"ready"`
}

type ScyllaNodeConfigStatus struct {
	// observedGeneration indicates the most recent generation observed by the ScyllaNodeConfig controller.
	ObservedGeneration int64 `json:"observedGeneration"`

	// conditions represents the latest available observations of a ScyllaNodeConfig's current state.
	Conditions []Condition `json:"conditions,omitempty"`

	// current represents status of current replicas.
	Current ReplicaStatus `json:"current"`

	// updated represents status of update replicas.
	Updated ReplicaStatus `json:"updated"`

	// stale indicates that status was updated based on stale API objects.
	Stale bool `json:"stale"`
}

type ScyllaNodeConfigPlacement struct {
	// affinity is a group of affinity scheduling rules for Pods.
	Affinity corev1.Affinity `json:"affinity"`
	
	// tolerations specify Pod's tolerations.
	Tolerations []corev1.Toleration `json:"tolerations"`

	// nodeSelector is a selector which must be true for the Pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
    // +kubebuilder:default:={"scylla-operator.scylladb.com/node-pool":"scylla"}
    NodeSelector map[string]string `json:"nodeSelector"`
}

type ScyllaNodeConfigSpec struct {
	// placement contains scheduling rules for ScyllaNodeConfig Pods.
	Placement ScyllaNodeConfigPlacement `json:"placement"`

	// disableOptimizations controls if nodes matching placement requirements
	// are going to be optimized. Turning off optimizations on already optimized
	// Nodes does not revert changes.
	DisableOptimizations bool `json:"disableOptimizations"`
}
```

By default, optimizations will be enabled on nodes labeled with `scylla-operator.scylladb.com/node-pool` label with value `scylla`, although users may filter the nodes using the `NodeSelector`, `Affinity` and `Tolerations`.
To disable optimizations users have to change `DisableOptimizations` to `true`.

#### Orchestration

A default ScyllaNodeConfig will be created by the Operator. Admins can either change the default CR, or provide their own additional ScyllaNodeConfig with non-conflicting node selector.
All RBAC resources necessary for running ScyllaNodeConfig (ClusterRole, ServiceAccount, ClusterRoleBinding) will be reconciled by the Scylla Operator.

NodeConfig will be deployed as DaemonSet managed by a dedicated controller in the operator.

NodeConfig Pod will observe Pods with nodeName FieldSelector to get notifications about Pods creation/updates/evictions on the same node.
When Scylla Pod lands, tuning Pod lists containers running on the host to check allocated resources and calculates CPU masks for Scylla and IRQ handling.

Perftune script - used for tuning - is a Python script and has several dependencies, it will be run as Job using Scylla image which has all of them.
Scylla image will be taken from global ScyllaOperatorConfig spec. 
Perftune requires a couple of parameters:
* CPU mask for cores dedicated for IRQ - calculated based on resources allocated to Scylla Pods running on the same node.
* Path under which storage device is mounted - we will use host path where the `data` PVC is mounted. Extracted via CRI. 
Perftune Job will be managed by the controller in ScyllaNodeConfig Pod.

When perftune Job is completed, a ConfigMap is created in the Scylla Pod namespace to unblock the sidecar waiting for it.
In case when Scylla Pod Node is not targeted by ScyllaNodeConfig, Operator will be responsible for creating ConfigMap the sidecar is waiting for.
Operator is going to check if any ScyllaNodeConfig Pod should be on the same node where Scylla Pod is. If not, an empty ConfigMap is created.
ConfigMap is going to be created with following name pattern: `perftune-<pod-uid>`

Sidecar is going to watch for this ConfigMap before it starts Scylla process.

#### Cloud support

Most of the optimizations are common to each supported Cloud Provider.
Although on GKE disabling writeback cache helps a lot, and in order to change it an additional parameter is required in perftune script.
To discover Cloud we are running on, we are going to send requests from ScyllaNodeConfig Pod. On GKE a DNS query to `metadata.google.internal` will be verified, 
if it exists, it means the Pod is running on GKE.
Because ScyllaNodeConfig Pod is running on the same node where tuned Scylla is, the instance metadata will be correct.

Currently, only `--write-back-cache=false` is required on GKE. Other Clouds use script without any additional parameters.
This parameter is only available in >= 2021.0.x perftune, so it's only added if used perftune supports it.

#### IRQ Layout
When any Scylla Pod is running on the host, IRQs are going to be pinned to CPUs not used by Scylla.

In case when the last Scylla Pod is removed from the k8s node, perftune is going to set IRQ to all available CPUs, but it’s not going to cancel other optimizations done by the script.   

When irqbalancer is running on the host, script is banning CPUs assigned to handle IRQ inside irqbalancer config file, and initiates a restart.  
It requires a shared Host PID namespace for that.

### CPU assignment discovery

Perftune script requires a list of CPUs assigned for IRQ handling.  
We don’t know which CPUs are dedicated to Scylla until the Scylla container is running.
When Scylla Pod lands, tuning Pod lists containers running on the host to check allocated resources and calculates CPU masks for Scylla and IRQ handling.

Only single supported Cloud Provider (EKS) exposes cpuset via Container Runtime Interface.
ScyllaNodeConfig Pod is going to first check if Scylla container cpuset is available in CRI. If not, it will fallback to
manually reading it from cgroup virtual filesystem on host machine mounted as a volume.

#### RBAC

The DaemonSet is going to use separate ServiceAccount with ClusterRole with read permissions for Pods and write permissions for Events and ConfigMaps, Jobs.
RBAC resources will be reconciled by the Operator.

#### Labels and ownership

* DaemonSet:
  * Label for selecting objects - `scylla-operator.scylladb.com/node-config-name = snc.Name`
  * Ownership - ControllerRef to ScyllaNodeConfig object
* ConfigMap with Perftune result
  * Label for selecting objects - `scylla-operator.scylladb.com/node-config-name = snc.Name`
  * Ownership - OwnerReference to Scylla Pod object
* Job with perftune
  * Labels for selecting objects:
    * `scylla-operator.scylladb.com/node-config-name = snc.Name`
    * `scylla-operator.scylladb.com/node-config-controller = nodeName`
  * Ownership - ControllerRef to ScyllaNodeConfig object

### Test Plan

We are not going to test if each individual optimization was applied to the host machine, but instead we are going to
observe if a corresponding ConfigMap is created for each Scylla Pod.

Unit tests are going to cover IRQ mask calculation and Pod cpuset discovery.
E2E are going to cover ConfigMaps and DaemonSet reconcilation.

### Upgrade / Downgrade Strategy

When the upgraded Operator see a missing resources added in this proposal it will create them.
All already existing ScyllaClusters available on Nodes which are subject for tuning are going to be automatically tuned and rolling restarted due to sidecar image change.

When Operator version is downgraded, resources created by Operator will not be automatically removed. 
Depending on whether ScyllaNodeConfig CRD was removed or not, some of them may be orphaned and eventually removed by GC.
When downgrade happens in the middle of ScyllaCluster rolling update, Scylla Pods may be stuck waiting for perftune result ConfigMap.
Because downgraded Operator doesn't know that it is supposed to create it, Pods will get stuck infinitely. To unblock the rollout, 
users have to manually remove Scylla Pods which are using upgraded sidecar version. StatefulSet controller will eventually downgrade them too.

### Version Skew Strategy

For the short period of time DaemonSet, Operator and sidecar version may differ. 
Operator will reconcile DaemonSet template to match Operator version, and DaemonSet controller is going to update all ScylaNodeConfig Pods to match required version.
Sidecar image is going to be updated to the same version Operator is using right after the update. 

## Drawbacks

It adds a highly privileged container to the k8s cluster, if anyone would be able to access it, it would be able to control host machine.

## Alternatives

Due to requirement of knowing which CPUs are dedicated to Scylla Pod, I thought about adding running perftune to sidecar.
Optimizations require access to host machine which is highly insecure for Pods exposed to end user so this proposal replaced sidecar idea. 
