// Copyright (C) 2021 ScyllaDB

package utils

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/mermaidclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	appv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	batchv1client "k8s.io/client-go/kubernetes/typed/batch/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/klog/v2"
)

func ContextForNodeConfigRollout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, nodeConfigRolloutTimeout)
}

func GetAvailableCondition(conditions []scyllav1alpha1.Condition) scyllav1alpha1.Condition {
	for _, c := range conditions {
		if c.Type == scyllav1alpha1.ScyllaNodeConfigAvailable {
			return c
		}
	}
	return scyllav1alpha1.Condition{}
}

func ScyllaNodeConfigRolledOut(snc *scyllav1alpha1.ScyllaNodeConfig) bool {
	return snc.Generation == snc.Status.ObservedGeneration &&
		GetAvailableCondition(snc.Status.Conditions).Status == corev1.ConditionTrue &&
		snc.Status.Current.Desired == snc.Status.Updated.Desired &&
		snc.Status.Current.Actual == snc.Status.Updated.Actual &&
		snc.Status.Current.Ready == snc.Status.Updated.Ready
}

func RolloutTimeoutForScyllaCluster(sc *v1.ScyllaCluster) time.Duration {
	return baseRolloutTimout + time.Duration(GetMemberCount(sc))*memberRolloutTimeout
}

func GetMemberCount(sc *v1.ScyllaCluster) int32 {
	members := int32(0)
	for _, r := range sc.Spec.Datacenter.Racks {
		members += r.Members
	}

	return members
}

func ContextForRollout(parent context.Context, sc *v1.ScyllaCluster) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, RolloutTimeoutForScyllaCluster(sc))
}

func SyncTimeoutForScyllaCluster(sc *v1.ScyllaCluster) time.Duration {
	tasks := int64(len(sc.Spec.Repairs) + len(sc.Spec.Backups))
	return baseManagerSyncTimeout + time.Duration(tasks)*managerTaskSyncTimeout
}

func ContextForManagerSync(parent context.Context, sc *v1.ScyllaCluster) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, SyncTimeoutForScyllaCluster(sc))
}

func ScyllaClusterRolledOut(sc *v1.ScyllaCluster) (bool, error) {
	// ObservedGeneration == nil will filter out the case when the object is initially created
	// so no other optional (but required) field should be nil after this point and we should error out.
	if sc.Status.ObservedGeneration == nil || *sc.Status.ObservedGeneration < sc.Generation {
		return false, nil
	}

	// TODO: this should be more straight forward - we need better status (conditions, aggregated state, ...)

	for _, r := range sc.Spec.Datacenter.Racks {
		rackStatus, found := sc.Status.Racks[r.Name]
		if !found {
			return false, nil
		}

		if rackStatus.Stale == nil {
			return true, fmt.Errorf("stale shouldn't be nil")
		}

		if *rackStatus.Stale {
			return false, nil
		}

		if rackStatus.Members != r.Members {
			return false, nil
		}

		if rackStatus.ReadyMembers != r.Members {
			return false, nil
		}

		if rackStatus.UpdatedMembers == nil {
			return true, fmt.Errorf("updatedMembers shouldn't be nil")
		}

		if *rackStatus.UpdatedMembers != r.Members {
			return false, nil
		}

		if rackStatus.Version != sc.Spec.Version {
			return false, nil
		}
	}

	if sc.Status.Upgrade != nil && sc.Status.Upgrade.FromVersion != sc.Status.Upgrade.ToVersion {
		return false, nil
	}

	framework.Infof("ScyllaCluster %s (RV=%s) is rolled out", klog.KObj(sc), sc.ResourceVersion)

	return true, nil
}

func WaitForScyllaClusterState(ctx context.Context, client scyllav1client.ScyllaV1Interface, namespace string, name string, conditions ...func(sc *v1.ScyllaCluster) (bool, error)) (*v1.ScyllaCluster, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.ScyllaClusters(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.ScyllaClusters(namespace).Watch(ctx, options)
		},
	}

	event, err := watchtools.UntilWithSync(ctx, lw, &v1.ScyllaCluster{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			for _, condition := range conditions {
				done, err := condition(event.Object.(*v1.ScyllaCluster))
				if err != nil {
					return false, err
				}
				if !done {
					return false, nil
				}
			}
			return true, nil
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}

	return event.Object.(*v1.ScyllaCluster), nil
}

type WaitForStateOptions struct {
	TolerateDelete bool
}

func WaitForPodState(ctx context.Context, client corev1client.CoreV1Interface, namespace string, name string, condition func(sc *corev1.Pod) (bool, error), o WaitForStateOptions) (*corev1.Pod, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.Pods(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.Pods(namespace).Watch(ctx, options)
		},
	}
	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return condition(event.Object.(*corev1.Pod))
		case watch.Deleted:
			if o.TolerateDelete {
				return condition(event.Object.(*corev1.Pod))
			}
			fallthrough
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*corev1.Pod), nil
}

func WaitForPVCState(ctx context.Context, client corev1client.CoreV1Interface, namespace string, name string, condition func(sc *corev1.PersistentVolumeClaim) (bool, error), o WaitForStateOptions) (*corev1.PersistentVolumeClaim, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.PersistentVolumeClaims(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.PersistentVolumeClaims(namespace).Watch(ctx, options)
		},
	}
	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.PersistentVolumeClaim{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return condition(event.Object.(*corev1.PersistentVolumeClaim))
		case watch.Deleted:
			if o.TolerateDelete {
				return condition(event.Object.(*corev1.PersistentVolumeClaim))
			}
			fallthrough
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*corev1.PersistentVolumeClaim), nil
}

func GetStatefulSetsForScyllaCluster(ctx context.Context, client appv1client.AppsV1Interface, sc *v1.ScyllaCluster) (map[string]*appsv1.StatefulSet, error) {
	statefulsetList, err := client.StatefulSets(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			naming.ClusterNameLabel: sc.Name,
		}.AsSelector().String(),
	})
	if err != nil {
		return nil, err
	}

	res := map[string]*appsv1.StatefulSet{}
	for _, s := range statefulsetList.Items {
		controllerRef := metav1.GetControllerOfNoCopy(&s)
		if controllerRef == nil {
			continue
		}

		if controllerRef.UID != sc.UID {
			continue
		}

		rackName := s.Labels[naming.RackNameLabel]
		res[rackName] = &s
	}

	return res, nil
}

func GetScyllaClient(ctx context.Context, client corev1client.CoreV1Interface, sc *v1.ScyllaCluster) (*scyllaclient.Client, []string, error) {
	hosts, err := GetHosts(ctx, client, sc)
	if err != nil {
		return nil, nil, err
	}

	if len(hosts) < 1 {
		return nil, nil, fmt.Errorf("no services found")
	}

	tokenSecret, err := client.Secrets(sc.Namespace).Get(ctx, naming.AgentAuthTokenSecretName(sc.Name), metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	authToken, err := helpers.GetAgentAuthTokenFromSecret(tokenSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("can't get auth token: %w", err)
	}

	// TODO: unify logging
	logger, _ := log.NewProduction(log.Config{
		Level: zap.NewAtomicLevelAt(zapcore.InfoLevel),
	})
	cfg := scyllaclient.DefaultConfig(authToken, hosts...)

	scyllaClient, err := scyllaclient.NewClient(cfg, logger.Named("scylla_client"))
	if err != nil {
		return nil, nil, err
	}

	return scyllaClient, hosts, nil
}

func GetHosts(ctx context.Context, client corev1client.CoreV1Interface, sc *v1.ScyllaCluster) ([]string, error) {
	serviceList, err := client.Services(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			naming.ClusterNameLabel: sc.Name,
		}.AsSelector().String(),
	})
	if err != nil {
		return nil, err
	}

	var hosts []string
	for _, s := range serviceList.Items {
		if s.Spec.Type != corev1.ServiceTypeClusterIP {
			return nil, fmt.Errorf("service %s/%s is of type %q instead of %q", s.Namespace, s.Name, s.Spec.Type, corev1.ServiceTypeClusterIP)
		}

		// TODO: Fix labeling in the operator so it can be selected.
		if s.Name == naming.HeadlessServiceNameForCluster(sc) {
			continue
		}

		if s.Spec.ClusterIP == corev1.ClusterIPNone {
			return nil, fmt.Errorf("service %s/%s doesn't have a ClusterIP", s.Namespace, s.Name)
		}

		hosts = append(hosts, s.Spec.ClusterIP)
	}

	return hosts, nil
}

// GetManagerClient gets managerClient using IP address. E2E tests shouldn't rely on InCluster DNS.
func GetManagerClient(ctx context.Context, client corev1client.CoreV1Interface) (*mermaidclient.Client, error) {
	managerService, err := client.Services(naming.ScyllaManagerNamespace).Get(ctx, naming.ScyllaManagerServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if managerService.Spec.ClusterIP == corev1.ClusterIPNone {
		return nil, fmt.Errorf("service %s/%s doesn't have a ClusterIP", managerService.Namespace, managerService.Name)
	}
	apiAddress := (&url.URL{
		Scheme: "http",
		Host:   managerService.Spec.ClusterIP,
		Path:   "/api/v1",
	}).String()

	manager, err := mermaidclient.NewClient(apiAddress, &http.Transport{})
	if err != nil {
		return nil, fmt.Errorf("create manager client, %w", err)
	}

	return &manager, nil
}

func GetNodeName(sc *v1.ScyllaCluster, idx int) string {
	return fmt.Sprintf(
		"%s-%s-%s-%d",
		sc.Name,
		sc.Spec.Datacenter.Name,
		sc.Spec.Datacenter.Racks[0].Name,
		idx,
	)
}

func WaitForScyllaNodeConfigRollout(ctx context.Context, client scyllav1alpha1client.ScyllaV1alpha1Interface, name string) (*scyllav1alpha1.ScyllaNodeConfig, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.ScyllaNodeConfigs().List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.ScyllaNodeConfigs().Watch(ctx, options)
		},
	}

	event, err := watchtools.UntilWithSync(ctx, lw, &scyllav1alpha1.ScyllaNodeConfig{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			snc := event.Object.(*scyllav1alpha1.ScyllaNodeConfig)
			return ScyllaNodeConfigRolledOut(snc), nil
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}

	return event.Object.(*scyllav1alpha1.ScyllaNodeConfig), nil
}
func WaitForConfigMap(ctx context.Context, client corev1client.CoreV1Interface, namespace, name string, o WaitForStateOptions, conditions ...func(*corev1.ConfigMap) (bool, error)) (*corev1.ConfigMap, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.ConfigMaps(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.ConfigMaps(namespace).Watch(ctx, options)
		},
	}

	checkConditions := func(conditions ...func(*corev1.ConfigMap) (bool, error)) func(*corev1.ConfigMap) (bool, error) {
		return func(configMap *corev1.ConfigMap) (bool, error) {
			for _, condition := range conditions {
				done, err := condition(configMap)
				if err != nil {
					return false, err
				}
				if !done {
					return false, nil
				}
			}
			return true, nil
		}
	}

	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.ConfigMap{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return checkConditions(conditions...)(event.Object.(*corev1.ConfigMap))
		case watch.Deleted:
			if o.TolerateDelete {
				return checkConditions(conditions...)(event.Object.(*corev1.ConfigMap))
			}
			fallthrough
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}

	return event.Object.(*corev1.ConfigMap), nil
}

func WaitForJob(ctx context.Context, client batchv1client.BatchV1Interface, namespace, name string, o WaitForStateOptions, conditions ...func(job *batchv1.Job) (bool, error)) (*batchv1.Job, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.Jobs(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.Jobs(namespace).Watch(ctx, options)
		},
	}

	checkConditions := func(conditions ...func(*batchv1.Job) (bool, error)) func(*batchv1.Job) (bool, error) {
		return func(job *batchv1.Job) (bool, error) {
			for _, condition := range conditions {
				done, err := condition(job)
				if err != nil {
					return false, err
				}
				if !done {
					return false, nil
				}
			}
			return true, nil
		}
	}

	event, err := watchtools.UntilWithSync(ctx, lw, &batchv1.Job{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return checkConditions(conditions...)(event.Object.(*batchv1.Job))
		case watch.Deleted:
			if o.TolerateDelete {
				return checkConditions(conditions...)(event.Object.(*batchv1.Job))
			}
			fallthrough
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}

	return event.Object.(*batchv1.Job), nil
}
