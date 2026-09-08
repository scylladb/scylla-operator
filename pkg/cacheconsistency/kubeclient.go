package cacheconsistency

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1ac "k8s.io/client-go/applyconfigurations/apps/v1"
	autoscalingv1ac "k8s.io/client-go/applyconfigurations/autoscaling/v1"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	networkingv1ac "k8s.io/client-go/applyconfigurations/networking/v1"
	policyv1ac "k8s.io/client-go/applyconfigurations/policy/v1"
	rbacv1ac "k8s.io/client-go/applyconfigurations/rbac/v1"
	"k8s.io/client-go/kubernetes"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	batchv1client "k8s.io/client-go/kubernetes/typed/batch/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	networkingv1client "k8s.io/client-go/kubernetes/typed/networking/v1"
	policyv1client "k8s.io/client-go/kubernetes/typed/policy/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
)

// NewRecordingKubeClient returns a kubernetes.Interface that records every write it makes to the kinds registered in store,
// so that WaitReady on the store covers them. Kinds not registered in the store, and kinds this client doesn't wrap, pass
// through unrecorded, and so does DeleteCollection.
//
// The wrapped kinds are Pods, Services, Secrets, ConfigMaps, ServiceAccounts, StatefulSets, Jobs, Ingresses,
// PodDisruptionBudgets and RoleBindings.
func NewRecordingKubeClient(client kubernetes.Interface, store *ConsistencyStore) kubernetes.Interface {
	return &kubeClient{
		Interface: client,
		store:     store,
	}
}

type kubeClient struct {
	kubernetes.Interface
	store *ConsistencyStore
}

func (c *kubeClient) CoreV1() corev1client.CoreV1Interface {
	return &coreV1Client{
		CoreV1Interface: c.Interface.CoreV1(),
		store:           c.store,
	}
}

func (c *kubeClient) AppsV1() appsv1client.AppsV1Interface {
	return &appsV1Client{
		AppsV1Interface: c.Interface.AppsV1(),
		store:           c.store,
	}
}

func (c *kubeClient) BatchV1() batchv1client.BatchV1Interface {
	return &batchV1Client{
		BatchV1Interface: c.Interface.BatchV1(),
		store:            c.store,
	}
}

func (c *kubeClient) NetworkingV1() networkingv1client.NetworkingV1Interface {
	return &networkingV1Client{
		NetworkingV1Interface: c.Interface.NetworkingV1(),
		store:                 c.store,
	}
}

func (c *kubeClient) PolicyV1() policyv1client.PolicyV1Interface {
	return &policyV1Client{
		PolicyV1Interface: c.Interface.PolicyV1(),
		store:             c.store,
	}
}

func (c *kubeClient) RbacV1() rbacv1client.RbacV1Interface {
	return &rbacV1Client{
		RbacV1Interface: c.Interface.RbacV1(),
		store:           c.store,
	}
}

type coreV1Client struct {
	corev1client.CoreV1Interface
	store *ConsistencyStore
}

func (c *coreV1Client) Pods(namespace string) corev1client.PodInterface {
	pods := c.CoreV1Interface.Pods(namespace)
	return &recordingPods{
		RecordingClientWithApplyAndStatus: NewRecordingClientWithApplyAndStatus[*corev1.Pod, *corev1.PodList, *corev1ac.PodApplyConfiguration](pods, namespace, c.store),
		PodExpansion:                      pods,
		pods:                              pods,
	}
}

// recordingPods adds the Pod-specific verbs to the generic client. Evictions are recorded as deletes.
type recordingPods struct {
	*RecordingClientWithApplyAndStatus[*corev1.Pod, *corev1.PodList, *corev1ac.PodApplyConfiguration]
	corev1client.PodExpansion
	pods corev1client.PodInterface
}

func (c *recordingPods) UpdateEphemeralContainers(ctx context.Context, podName string, pod *corev1.Pod, opts metav1.UpdateOptions) (*corev1.Pod, error) {
	updated, err := c.pods.UpdateEphemeralContainers(ctx, podName, pod, opts)
	return observeWrite(c.recorder, updated, err)
}

func (c *recordingPods) UpdateResize(ctx context.Context, podName string, pod *corev1.Pod, opts metav1.UpdateOptions) (*corev1.Pod, error) {
	updated, err := c.pods.UpdateResize(ctx, podName, pod, opts)
	return observeWrite(c.recorder, updated, err)
}

func (c *recordingPods) Evict(ctx context.Context, eviction *policyv1beta1.Eviction) error {
	return c.recorder.observeDelete(eviction.Name, c.PodExpansion.Evict(ctx, eviction))
}

func (c *recordingPods) EvictV1(ctx context.Context, eviction *policyv1.Eviction) error {
	return c.recorder.observeDelete(eviction.Name, c.PodExpansion.EvictV1(ctx, eviction))
}

func (c *recordingPods) EvictV1beta1(ctx context.Context, eviction *policyv1beta1.Eviction) error {
	return c.recorder.observeDelete(eviction.Name, c.PodExpansion.EvictV1beta1(ctx, eviction))
}

func (c *coreV1Client) Services(namespace string) corev1client.ServiceInterface {
	services := c.CoreV1Interface.Services(namespace)
	return &recordingServices{
		RecordingClientWithApplyAndStatus: NewRecordingClientWithApplyAndStatus[*corev1.Service, *corev1.ServiceList, *corev1ac.ServiceApplyConfiguration](
			serviceClient{ServiceInterface: services}, namespace, c.store,
		),
		ServiceExpansion: services,
	}
}

// serviceClient completes the Service client to the generic method set: Services are the one kind without
// DeleteCollection, so it is rejected here and stays unreachable through the ServiceInterface.
type serviceClient struct {
	corev1client.ServiceInterface
}

func (serviceClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return apierrors.NewMethodNotSupported(corev1.Resource("services"), "deletecollection")
}

// recordingServices adds the Service-specific verbs to the generic client.
type recordingServices struct {
	*RecordingClientWithApplyAndStatus[*corev1.Service, *corev1.ServiceList, *corev1ac.ServiceApplyConfiguration]
	corev1client.ServiceExpansion
}

func (c *coreV1Client) Secrets(namespace string) corev1client.SecretInterface {
	return NewRecordingClientWithApply[*corev1.Secret, *corev1.SecretList, *corev1ac.SecretApplyConfiguration](c.CoreV1Interface.Secrets(namespace), namespace, c.store)
}

func (c *coreV1Client) ConfigMaps(namespace string) corev1client.ConfigMapInterface {
	return NewRecordingClientWithApply[*corev1.ConfigMap, *corev1.ConfigMapList, *corev1ac.ConfigMapApplyConfiguration](c.CoreV1Interface.ConfigMaps(namespace), namespace, c.store)
}

func (c *coreV1Client) ServiceAccounts(namespace string) corev1client.ServiceAccountInterface {
	serviceAccounts := c.CoreV1Interface.ServiceAccounts(namespace)
	return &recordingServiceAccounts{
		RecordingClientWithApply: NewRecordingClientWithApply[*corev1.ServiceAccount, *corev1.ServiceAccountList, *corev1ac.ServiceAccountApplyConfiguration](serviceAccounts, namespace, c.store),
		serviceAccounts:          serviceAccounts,
	}
}

// recordingServiceAccounts adds the ServiceAccount-specific verbs to the generic client. Token requests don't change
// the ServiceAccount, so they pass through unrecorded.
type recordingServiceAccounts struct {
	*RecordingClientWithApply[*corev1.ServiceAccount, *corev1.ServiceAccountList, *corev1ac.ServiceAccountApplyConfiguration]
	serviceAccounts corev1client.ServiceAccountInterface
}

func (c *recordingServiceAccounts) CreateToken(ctx context.Context, serviceAccountName string, tokenRequest *authenticationv1.TokenRequest, opts metav1.CreateOptions) (*authenticationv1.TokenRequest, error) {
	return c.serviceAccounts.CreateToken(ctx, serviceAccountName, tokenRequest, opts)
}

type appsV1Client struct {
	appsv1client.AppsV1Interface
	store *ConsistencyStore
}

func (c *appsV1Client) StatefulSets(namespace string) appsv1client.StatefulSetInterface {
	statefulSets := c.AppsV1Interface.StatefulSets(namespace)
	return &recordingStatefulSets{
		RecordingClientWithApplyAndStatus: NewRecordingClientWithApplyAndStatus[*appsv1.StatefulSet, *appsv1.StatefulSetList, *appsv1ac.StatefulSetApplyConfiguration](statefulSets, namespace, c.store),
		statefulSets:                      statefulSets,
	}
}

// recordingStatefulSets adds the scale subresource verbs to the generic client. The scale subresource carries the
// resourceVersion of the StatefulSet it scaled, so scale writes are recorded against the StatefulSet.
type recordingStatefulSets struct {
	*RecordingClientWithApplyAndStatus[*appsv1.StatefulSet, *appsv1.StatefulSetList, *appsv1ac.StatefulSetApplyConfiguration]
	statefulSets appsv1client.StatefulSetInterface
}

func (c *recordingStatefulSets) GetScale(ctx context.Context, statefulSetName string, opts metav1.GetOptions) (*autoscalingv1.Scale, error) {
	return c.statefulSets.GetScale(ctx, statefulSetName, opts)
}

func (c *recordingStatefulSets) UpdateScale(ctx context.Context, statefulSetName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error) {
	updated, err := c.statefulSets.UpdateScale(ctx, statefulSetName, scale, opts)
	return observeWrite(c.recorder, updated, err)
}

func (c *recordingStatefulSets) ApplyScale(ctx context.Context, statefulSetName string, scale *autoscalingv1ac.ScaleApplyConfiguration, opts metav1.ApplyOptions) (*autoscalingv1.Scale, error) {
	applied, err := c.statefulSets.ApplyScale(ctx, statefulSetName, scale, opts)
	return observeWrite(c.recorder, applied, err)
}

type batchV1Client struct {
	batchv1client.BatchV1Interface
	store *ConsistencyStore
}

func (c *batchV1Client) Jobs(namespace string) batchv1client.JobInterface {
	return NewRecordingClientWithApplyAndStatus[*batchv1.Job, *batchv1.JobList, *batchv1ac.JobApplyConfiguration](c.BatchV1Interface.Jobs(namespace), namespace, c.store)
}

type networkingV1Client struct {
	networkingv1client.NetworkingV1Interface
	store *ConsistencyStore
}

func (c *networkingV1Client) Ingresses(namespace string) networkingv1client.IngressInterface {
	return NewRecordingClientWithApplyAndStatus[*networkingv1.Ingress, *networkingv1.IngressList, *networkingv1ac.IngressApplyConfiguration](c.NetworkingV1Interface.Ingresses(namespace), namespace, c.store)
}

type policyV1Client struct {
	policyv1client.PolicyV1Interface
	store *ConsistencyStore
}

func (c *policyV1Client) PodDisruptionBudgets(namespace string) policyv1client.PodDisruptionBudgetInterface {
	return NewRecordingClientWithApplyAndStatus[*policyv1.PodDisruptionBudget, *policyv1.PodDisruptionBudgetList, *policyv1ac.PodDisruptionBudgetApplyConfiguration](c.PolicyV1Interface.PodDisruptionBudgets(namespace), namespace, c.store)
}

type rbacV1Client struct {
	rbacv1client.RbacV1Interface
	store *ConsistencyStore
}

func (c *rbacV1Client) RoleBindings(namespace string) rbacv1client.RoleBindingInterface {
	return NewRecordingClientWithApply[*rbacv1.RoleBinding, *rbacv1.RoleBindingList, *rbacv1ac.RoleBindingApplyConfiguration](c.RbacV1Interface.RoleBindings(namespace), namespace, c.store)
}
