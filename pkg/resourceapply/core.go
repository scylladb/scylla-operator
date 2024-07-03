package resourceapply

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
)

func ApplyConfigMapWithControl(
	ctx context.Context,
	control ApplyControlInterface[*corev1.ConfigMap],
	recorder record.EventRecorder,
	required *corev1.ConfigMap,
	options ApplyOptions,
) (*corev1.ConfigMap, bool, error) {
	return ApplyGeneric[*corev1.ConfigMap](ctx, control, recorder, required, options)
}

func ApplyConfigMap(
	ctx context.Context,
	client corev1client.ConfigMapsGetter,
	lister corev1listers.ConfigMapLister,
	recorder record.EventRecorder,
	required *corev1.ConfigMap,
	options ApplyOptions,
) (*corev1.ConfigMap, bool, error) {
	return ApplyConfigMapWithControl(
		ctx,
		ApplyControlFuncs[*corev1.ConfigMap]{
			GetCachedFunc: lister.ConfigMaps(required.Namespace).Get,
			CreateFunc:    client.ConfigMaps(required.Namespace).Create,
			UpdateFunc:    client.ConfigMaps(required.Namespace).Update,
			DeleteFunc:    client.ConfigMaps(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplySecretWithControl(
	ctx context.Context,
	control ApplyControlInterface[*corev1.Secret],
	recorder record.EventRecorder,
	required *corev1.Secret,
	options ApplyOptions,
) (*corev1.Secret, bool, error) {
	return ApplyGeneric[*corev1.Secret](ctx, control, recorder, required, options)
}

func ApplySecret(
	ctx context.Context,
	client corev1client.SecretsGetter,
	lister corev1listers.SecretLister,
	recorder record.EventRecorder,
	required *corev1.Secret,
	options ApplyOptions,
) (*corev1.Secret, bool, error) {
	return ApplySecretWithControl(
		ctx,
		ApplyControlFuncs[*corev1.Secret]{
			GetCachedFunc: lister.Secrets(required.Namespace).Get,
			CreateFunc:    client.Secrets(required.Namespace).Create,
			UpdateFunc:    client.Secrets(required.Namespace).Update,
			DeleteFunc:    client.Secrets(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyServiceWithControl(
	ctx context.Context,
	control ApplyControlInterface[*corev1.Service],
	recorder record.EventRecorder,
	required *corev1.Service,
	options ApplyOptions,
) (*corev1.Service, bool, error) {
	return ApplyGenericWithHandlers[*corev1.Service](
		ctx,
		control,
		recorder,
		required,
		options,
		func(required **corev1.Service, existing *corev1.Service) {
			(*required).Spec.ClusterIP = existing.Spec.ClusterIP
			(*required).Spec.ClusterIPs = existing.Spec.ClusterIPs
		},
		nil,
	)
}

func ApplyService(
	ctx context.Context,
	client corev1client.ServicesGetter,
	lister corev1listers.ServiceLister,
	recorder record.EventRecorder,
	required *corev1.Service,
	options ApplyOptions,
) (*corev1.Service, bool, error) {
	return ApplyServiceWithControl(
		ctx,
		ApplyControlFuncs[*corev1.Service]{
			GetCachedFunc: lister.Services(required.Namespace).Get,
			CreateFunc:    client.Services(required.Namespace).Create,
			UpdateFunc:    client.Services(required.Namespace).Update,
			DeleteFunc:    client.Services(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyServiceAccountWithControl(
	ctx context.Context,
	control ApplyControlInterface[*corev1.ServiceAccount],
	recorder record.EventRecorder,
	required *corev1.ServiceAccount,
	options ApplyOptions,
) (*corev1.ServiceAccount, bool, error) {
	return ApplyGeneric[*corev1.ServiceAccount](ctx, control, recorder, required, options)
}

func ApplyServiceAccount(
	ctx context.Context,
	client corev1client.ServiceAccountsGetter,
	lister corev1listers.ServiceAccountLister,
	recorder record.EventRecorder,
	required *corev1.ServiceAccount,
	options ApplyOptions,
) (*corev1.ServiceAccount, bool, error) {
	return ApplyServiceAccountWithControl(
		ctx,
		ApplyControlFuncs[*corev1.ServiceAccount]{
			GetCachedFunc: lister.ServiceAccounts(required.Namespace).Get,
			CreateFunc:    client.ServiceAccounts(required.Namespace).Create,
			UpdateFunc:    client.ServiceAccounts(required.Namespace).Update,
			DeleteFunc:    client.ServiceAccounts(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyNamespaceWithControl(
	ctx context.Context,
	control ApplyControlInterface[*corev1.Namespace],
	recorder record.EventRecorder,
	required *corev1.Namespace,
	options ApplyOptions,
) (*corev1.Namespace, bool, error) {
	return ApplyGeneric[*corev1.Namespace](ctx, control, recorder, required, options)
}

func ApplyNamespace(
	ctx context.Context,
	client corev1client.NamespacesGetter,
	lister corev1listers.NamespaceLister,
	recorder record.EventRecorder,
	required *corev1.Namespace,
	options ApplyOptions,
) (*corev1.Namespace, bool, error) {
	return ApplyNamespaceWithControl(
		ctx,
		ApplyControlFuncs[*corev1.Namespace]{
			GetCachedFunc: lister.Get,
			CreateFunc:    client.Namespaces().Create,
			UpdateFunc:    client.Namespaces().Update,
			DeleteFunc:    client.Namespaces().Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyEndpointsWithControl(
	ctx context.Context,
	control ApplyControlInterface[*corev1.Endpoints],
	recorder record.EventRecorder,
	required *corev1.Endpoints,
	options ApplyOptions,
) (*corev1.Endpoints, bool, error) {
	return ApplyGeneric[*corev1.Endpoints](ctx, control, recorder, required, options)
}

func ApplyEndpoints(
	ctx context.Context,
	client corev1client.EndpointsGetter,
	lister corev1listers.EndpointsLister,
	recorder record.EventRecorder,
	required *corev1.Endpoints,
	options ApplyOptions,
) (*corev1.Endpoints, bool, error) {
	return ApplyEndpointsWithControl(
		ctx,
		ApplyControlFuncs[*corev1.Endpoints]{
			GetCachedFunc: lister.Endpoints(required.Namespace).Get,
			CreateFunc:    client.Endpoints(required.Namespace).Create,
			UpdateFunc:    client.Endpoints(required.Namespace).Update,
			DeleteFunc:    client.Endpoints(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyPodWithControl(
	ctx context.Context,
	control ApplyControlInterface[*corev1.Pod],
	recorder record.EventRecorder,
	required *corev1.Pod,
	options ApplyOptions,
) (*corev1.Pod, bool, error) {
	return ApplyGeneric[*corev1.Pod](ctx, control, recorder, required, options)
}

func ApplyPod(
	ctx context.Context,
	client corev1client.PodsGetter,
	lister corev1listers.PodLister,
	recorder record.EventRecorder,
	required *corev1.Pod,
	options ApplyOptions,
) (*corev1.Pod, bool, error) {
	return ApplyPodWithControl(
		ctx,
		ApplyControlFuncs[*corev1.Pod]{
			GetCachedFunc: lister.Pods(required.Namespace).Get,
			CreateFunc:    client.Pods(required.Namespace).Create,
			UpdateFunc:    client.Pods(required.Namespace).Update,
			DeleteFunc:    client.Pods(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}
