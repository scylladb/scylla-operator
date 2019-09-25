package util

import (
	"context"

	"go.uber.org/zap/zapcore"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
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
func LoggerForCluster(c *scyllav1alpha1.Cluster) log.Logger {
	l, _ := log.NewProduction(log.Config{
		Level: zapcore.DebugLevel,
	})
	return l.With("cluster", c.Namespace+"/"+c.Name, "resourceVersion", c.ResourceVersion)
}

// StatefulSetStatusesStale checks if the StatefulSet Objects of a Cluster
// have been observed by the StatefulSet controller.
// If they haven't, their status might be stale, so it's better to wait
// and process them later.
func AreStatefulSetStatusesStale(ctx context.Context, c *scyllav1alpha1.Cluster, client client.Client) (bool, error) {
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

func GetMemberServicesForRack(ctx context.Context, r scyllav1alpha1.RackSpec, c *scyllav1alpha1.Cluster, cl client.Client) ([]corev1.Service, error) {
	svcList := &corev1.ServiceList{}
	err := cl.List(ctx, &client.ListOptions{
		LabelSelector: naming.RackSelector(r, c),
	}, svcList)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list member services")
	}
	return svcList.Items, nil
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
func NewControllerRef(c *scyllav1alpha1.Cluster) metav1.OwnerReference {
	return *metav1.NewControllerRef(c, schema.GroupVersionKind{
		Group:   "scylla.scylladb.com",
		Version: "v1alpha1",
		Kind:    "Cluster",
	})
}
