package naming

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func StatefulSetNameForRack(r scyllav1.RackSpec, c *scyllav1.ScyllaCluster) string {
	return fmt.Sprintf("%s-%s-%s", c.Name, c.Spec.Datacenter.Name, r.Name)
}

func ServiceNameFromPod(pod *corev1.Pod) string {
	// Pod and Service has the same name
	return pod.Name
}

func MemberServiceName(r scyllav1.RackSpec, c *scyllav1.ScyllaCluster, idx int) string {
	return fmt.Sprintf("%s-%d", StatefulSetNameForRack(r, c), idx)
}

func ServiceDNSName(service string, c *scyllav1.ScyllaCluster) string {
	return fmt.Sprintf("%s.%s", service, CrossNamespaceServiceNameForCluster(c))
}

func ServiceAccountNameForMembers(c *scyllav1.ScyllaCluster) string {
	return fmt.Sprintf("%s-member", c.Name)
}

func HeadlessServiceNameForCluster(c *scyllav1.ScyllaCluster) string {
	return fmt.Sprintf("%s-client", c.Name)
}

func CrossNamespaceServiceNameForCluster(c *scyllav1.ScyllaCluster) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", HeadlessServiceNameForCluster(c), c.Namespace)
}

func ManagerClusterName(c *scyllav1.ScyllaCluster) string {
	return c.Namespace + "/" + c.Name
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

func ImageToVersion(image string) (string, error) {
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

// ScyllaImage returns version of Scylla container.
func ScyllaImage(containers []corev1.Container) (string, error) {
	idx, err := FindScyllaContainer(containers)
	if err != nil {
		return "", errors.Wrap(err, "find scylla container")
	}

	version, err := ImageToVersion(containers[idx].Image)
	if err != nil {
		return "", errors.Wrap(err, "parse scylla container version")
	}
	return version, nil
}
