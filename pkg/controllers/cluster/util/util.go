package util

import (
	"context"

	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
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

// GetStatefulSetForRack returns rack underlying StatefulSet.
func GetStatefulSetForRack(ctx context.Context, rack scyllav1.RackSpec, cluster *scyllav1.ScyllaCluster, kubeClient kubernetes.Interface) (*appsv1.StatefulSet, error) {
	stsName := naming.StatefulSetNameForRack(rack, cluster)
	sts, err := kubeClient.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, stsName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return sts, nil
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
