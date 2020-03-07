package naming

import (
	"fmt"
	"github.com/pkg/errors"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

func NamespacedName(name, namespace string) client.ObjectKey {
	return client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}
}

func NamespacedNameForObject(obj metav1.Object) client.ObjectKey {
	return client.ObjectKey{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
}

func StatefulSetNameForRack(r scyllav1alpha1.RackSpec, c *scyllav1alpha1.Cluster) string {
	return fmt.Sprintf("%s-%s-%s", c.Name, c.Spec.Datacenter.Name, r.Name)
}

func ServiceAccountNameForMembers(c *scyllav1alpha1.Cluster) string {
	return fmt.Sprintf("%s-member", c.Name)
}

func HeadlessServiceNameForCluster(c *scyllav1alpha1.Cluster) string {
	return fmt.Sprintf("%s-client", c.Name)
}

func PVCNameForPod(podName string) string {
	return fmt.Sprintf("%s-%s", PVCTemplateName, podName)
}

// IndexFromName attempts to get the index from a name using the
// naming convention <name>-<index>.
func IndexFromName(n string) (int32, error) {

	// index := svc.Name[strings.LastIndex(svc.Name, "-") + 1 : len(svc.Name)]
	delimIndex := strings.LastIndex(n, "-")
	if delimIndex == -1 {
		return -1, errors.New(fmt.Sprintf("didn't find '-' delimiter in string %s", n))
	}

	index, err := strconv.Atoi(n[delimIndex+1:])
	if err != nil {
		return -1, errors.New(fmt.Sprintf("couldn't convert '%s' to a number", n[delimIndex+1:]))
	}

	return int32(index), nil
}

func ImageToVersion(image string) (string, error){
	parts := strings.Split(image, ":")
	if len(parts) != 2 || len(parts[1]) == 0 {
		return "", errors.New(fmt.Sprintf("Invalid image name: %s", image))
	}
	return parts[1], nil
}

func FindScyllaContainer(containers []corev1.Container) (int, error) {
	for idx := range containers {
		if containers[idx].Name == ScyllaContainerName {
			return idx, nil
		}
	}
	return -1, errors.New(fmt.Sprintf("Scylla Container '%s' not found", ScyllaContainerName))
}
