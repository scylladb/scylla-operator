package util

import (
	"context"
	"encoding/json"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	. "github.com/scylladb/scylla-operator/pkg/util/nodeaffinity"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"strconv"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LoggerForCluster returns a logger that will log with context
// about the current cluster
func LoggerForCluster(c *scyllav1.ScyllaCluster) log.Logger {
	l, _ := log.NewProduction(log.Config{
		Level: zapcore.DebugLevel,
	})
	return l.With("cluster", c.Namespace+"/"+c.Name, "resourceVersion", c.ResourceVersion)
}

// StatefulSetStatusesStale checks if the StatefulSet Objects of a Cluster
// have been observed by the StatefulSet controller.
// If they haven't, their status might be stale, so it's better to wait
// and process them later.
func AreStatefulSetStatusesStale(ctx context.Context, c *scyllav1.ScyllaCluster, client client.Client) (bool, error) {
	sts := &appsv1.StatefulSet{}
	for _, r := range c.Spec.Datacenter.Racks {
		err := client.Get(ctx, naming.NamespacedName(naming.StatefulSetNameForRack(r, c), c.Namespace), sts)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return true, errors.Wrap(err, "error getting statefulset")
		}
		if sts.Generation != sts.Status.ObservedGeneration {
			return true, nil
		}
	}
	return false, nil
}

func GetMemberServicesForRack(ctx context.Context, r scyllav1.RackSpec, c *scyllav1.ScyllaCluster, cl client.Client) ([]corev1.Service, error) {
	svcList := &corev1.ServiceList{}
	err := cl.List(ctx, svcList, &client.ListOptions{
		LabelSelector: naming.RackSelector(r, c),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list member services")
	}
	return svcList.Items, nil
}

func RackScalingDown(c *scyllav1.ScyllaCluster, r scyllav1.RackSpec) bool {
	statusMembers := c.Status.Racks[r.Name].Members
	specMembers := r.Members
	return specMembers < statusMembers
}

func RackReady(c *scyllav1.ScyllaCluster, r scyllav1.RackSpec) bool {
	rackStatus := c.Status.Racks[r.Name]
	return rackStatus.Members == rackStatus.ReadyMembers
}

// WillContainLoad function requests storageService load for every pod running and checks if sum of loads
// is smaller than sum of capacities of rack members i.e. data will not be lost.
// Returns bool or error if one occurred.
func WillContainLoad(ctx context.Context, logger log.Logger, cluster *scyllav1.ScyllaCluster, rack scyllav1.RackSpec, scyllaClient *scyllaclient.Client, client client.Client) (bool, error) {
	capacity := int64(0)
	if capacityQuantity, err := resource.ParseQuantity(rack.Storage.Capacity); err != nil {
		return false, errors.Errorf("Parsing storage capacity error %s", err)
	} else {
		capacity = capacityQuantity.MilliValue()
	}

	// obtain bearer token for communication with scylla manager agent
	if bearerToken, err := obtainBearerToken(ctx, rack, client); err != nil {
		return false, errors.Errorf("Parsing storage capacity error %s", err)
	} else {
		scyllaClient.AddBearerToken(bearerToken)
	}

	// get load from every member
	statusMembers := cluster.Status.Racks[rack.Name].Members
	loadSum := int64(0)
	for i := 0; i < int(statusMembers); i++ {
		if load, err := memberLoad(ctx, cluster, rack, scyllaClient, i); err != nil {
			return false, err
		} else {
			loadSum += load
		}
	}

	// compare with current capacity
	if loadSum > int64(rack.Members)*capacity {
		logger.Error(ctx, "Unable to store current load in new rack spec", "current load", loadSum, "new capacity", int64(rack.Members)*capacity)
		return false, nil
	}
	return true, nil
}

// memberLoad function  makes a GET request on /storage_service/load for given host number
// Returns load or error if one occurred.
func memberLoad(ctx context.Context, c *scyllav1.ScyllaCluster, rack scyllav1.RackSpec, sc *scyllaclient.Client, idx int) (int64, error) {
	hostName := naming.StatefulSetNameForRack(rack, c) + "-" + strconv.Itoa(idx) + "." + naming.CrossNamespaceServiceNameForCluster(c)
	ctx = scyllaclient.ForceHostWrapper(ctx, hostName)

	value, err := sc.StorageServiceLoadGetWrapper(ctx)
	if err != nil {
		return -1, errors.Errorf("StorageServiceLoadGetWrapper error %s", err)
	}
	load, err := value.GetPayload().(json.Number).Int64()
	if err != nil {
		return -1, errors.Errorf("int64 from getPayload() error %s", err)
	}
	return load, nil
}

// obtainBearerToken extracts token from secret named like ScyllaAgentConfig field in rack.
// Returns empty string if secret not found.
func obtainBearerToken(ctx context.Context, rack scyllav1.RackSpec, cl client.Client) (string, error) {
	scyllaAgentConfig := rack.ScyllaAgentConfig
	secrets := &corev1.SecretList{}
	if err := cl.List(ctx, secrets); err != nil {
		return "", errors.Errorf("List secrets err: %s", err)
	}
	for _, secret := range secrets.Items {
		if secret.ObjectMeta.Name == scyllaAgentConfig {
			return string(secret.Data["token"]), nil
		}
	}
	return "", nil
}

// WillSchedule checks if every rack has a node to be scheduled on in terms of nodes' taints and matching racks' tolerations
// and racks' node affinities.
// Returns boolean and error if one occurred.
func WillSchedule(ctx context.Context, c *scyllav1.ScyllaCluster, cl client.Client) (bool, error) {
	nodes := &corev1.NodeList{}
	if err := cl.List(ctx, nodes); err != nil {
		return false, errors.Wrap(err, "list nodes")
	}
	// every rack has to have at least one node to be scheduled on
	for _, rack := range c.Spec.Datacenter.Racks {
		hasNodeToBeScheduledOn := false
		for _, node := range nodes.Items {
			if taintsPreventScheduling, nodeAffinityPreventsScheduling, err := checkTaintTolerationAndNodeAffinity(node, rack); err != nil {
				return false, err
			} else if !taintsPreventScheduling && !nodeAffinityPreventsScheduling {
				hasNodeToBeScheduledOn = true
			}
		}
		if !hasNodeToBeScheduledOn {
			return false, nil
		}
	}
	return true, nil
}

// checkTaintTolerationAndNodeAffinity checks taints and node affinity for given node
// Returns booleans if any taint prevents scheduling or node doesn't match node affinity, if present.
func checkTaintTolerationAndNodeAffinity(node corev1.Node, rack scyllav1.RackSpec) (bool, bool, error) {
	// check if every node's taint has matching toleration
	taintsPreventScheduling := false
	for _, taint := range node.Spec.Taints {
		if !taintTolerableByRack(taint, rack) {
			taintsPreventScheduling = true
		}
	}
	// checks if node affinity matches given node
	nodeAffinityPreventsScheduling := false
	if placement := rack.Placement; placement != nil {
		if nodeAffinity := placement.NodeAffinity; nodeAffinity != nil {
			// consider node affinity that cannot be ignored
			if nodeSelector := nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution; nodeSelector != nil {
				if ns, err := NewNodeSelector(nodeSelector); err != nil {
					return false, false, err
				} else if !ns.Match(&node) {
					nodeAffinityPreventsScheduling = true
				}
			}
		}
	}
	return taintsPreventScheduling, nodeAffinityPreventsScheduling, nil
}

func taintTolerableByRack(taint corev1.Taint, rack scyllav1.RackSpec) bool {
	// if taint prohibits scheduling/executing
	if taint.Effect == corev1.TaintEffectNoExecute || taint.Effect == corev1.TaintEffectNoSchedule {
		if rack.Placement != nil {
			// check if toleration matches taint
			for _, toleration := range rack.Placement.Tolerations {
				// key matches or key and value match
				if (toleration.Operator == corev1.TolerationOpExists && (toleration.Key == taint.Key || toleration.Key == "")) ||
					(toleration.Operator == corev1.TolerationOpEqual && toleration.Key == taint.Key && toleration.Value == taint.Value) {
					return true
				}
			}
			return false
		} else {
			return false // no Placement means no toleration for taint
		}
	} else {
		return true // taint effect doesn't prohibit scheduling
	}
}

// RefFromString is a helper function that takes a string
// and outputs a reference to that string.
// Useful for initializing a string pointer from a literal.
func RefFromString(s string) *string {
	return &s
}

// RefFromInt32 is a helper function that takes a int32
// and outputs a reference to that int.
func RefFromInt32(i int32) *int32 {
	return &i
}

// VerifyOwner checks if the owner Object is the controller
// of the obj Object and returns an error if it isn't.
func VerifyOwner(obj, owner metav1.Object) error {
	if !metav1.IsControlledBy(obj, owner) {
		ownerRef := metav1.GetControllerOf(obj)
		return errors.Errorf(
			"'%s/%s' is foreign owned: "+
				"it is owned by '%v', not '%s/%s'.",
			obj.GetNamespace(), obj.GetName(),
			ownerRef,
			owner.GetNamespace(), owner.GetName(),
		)
	}
	return nil
}

// NewControllerRef returns an OwnerReference to
// the provided Cluster Object
func NewControllerRef(c *scyllav1.ScyllaCluster) metav1.OwnerReference {
	return *metav1.NewControllerRef(c, schema.GroupVersionKind{
		Group:   "scylla.scylladb.com",
		Version: "v1",
		Kind:    "ScyllaCluster",
	})
}

// MarkAsReplaceCandidate patches member service with special label indicating
// that service must be replaced.
func MarkAsReplaceCandidate(ctx context.Context, member *corev1.Service, kubeClient kubernetes.Interface) error {
	if _, ok := member.Labels[naming.ReplaceLabel]; !ok {
		patched := member.DeepCopy()
		patched.Labels[naming.ReplaceLabel] = ""
		if err := PatchService(ctx, member, patched, kubeClient); err != nil {
			return errors.Wrap(err, "patch service as replace")
		}
	}
	return nil
}
