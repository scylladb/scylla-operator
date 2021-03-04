package scyllacluster

import (
	"context"
	"fmt"
	"time"

	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	appv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

func rolloutTimeoutForScyllaCluster(sc *scyllav1.ScyllaCluster) time.Duration {
	members := int64(0)
	for _, r := range sc.Spec.Datacenter.Racks {
		members += int64(r.Members)
	}

	return baseRolloutTimout + time.Duration(members)*memberRolloutTimeout
}

func contextForRollout(parent context.Context, sc *scyllav1.ScyllaCluster) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, rolloutTimeoutForScyllaCluster(sc))
}

func scyllaClusterRolledOut(sc *scyllav1.ScyllaCluster) (bool, error) {
	// TODO: check observed generation when it's added.
	// TODO: this should be more straight forward - we need better status (conditions, aggregated state, ...)

	for _, r := range sc.Spec.Datacenter.Racks {
		if sc.Status.Racks[r.Name].ReadyMembers != r.Members {
			return false, nil
		}

		if sc.Status.Racks[r.Name].Version != sc.Spec.Version {
			return false, nil
		}
	}

	if sc.Status.Upgrade != nil && sc.Status.Upgrade.FromVersion != sc.Status.Upgrade.ToVersion {
		return false, nil
	}

	return true, nil
}

func waitForScyllaClusterState(ctx context.Context, client scyllav1client.ScyllaV1Interface, namespace string, name string, condition func(sc *scyllav1.ScyllaCluster) (bool, error)) (*scyllav1.ScyllaCluster, error) {
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
	event, err := watchtools.UntilWithSync(ctx, lw, &scyllav1.ScyllaCluster{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return condition(event.Object.(*scyllav1.ScyllaCluster))
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*scyllav1.ScyllaCluster), nil
}

type waitForStateOptions struct {
	tolerateDelete bool
}

func waitForPodState(ctx context.Context, client corev1client.CoreV1Interface, namespace string, name string, condition func(sc *corev1.Pod) (bool, error), o waitForStateOptions) (*corev1.Pod, error) {
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
			if o.tolerateDelete {
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

func getStatefulSetsForScyllaCluster(ctx context.Context, client appv1client.AppsV1Interface, sc *scyllav1.ScyllaCluster) (map[string]*appsv1.StatefulSet, error) {
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

func getScyllaClient(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) (*scyllaclient.Client, []string, error) {
	serviceList, err := client.Services(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			naming.ClusterNameLabel: sc.Name,
		}.AsSelector().String(),
	})
	if err != nil {
		return nil, nil, err
	}

	var hosts []string
	for _, s := range serviceList.Items {
		if s.Spec.Type != corev1.ServiceTypeClusterIP {
			return nil, nil, fmt.Errorf("service %s/%s is of type %q instead of %q", s.Namespace, s.Name, s.Spec.Type, corev1.ServiceTypeClusterIP)
		}

		// TODO: Fix labeling in the operator so it can be selected.
		if s.Name == naming.HeadlessServiceNameForCluster(sc) {
			continue
		}

		if s.Spec.ClusterIP == corev1.ClusterIPNone {
			return nil, nil, fmt.Errorf("service %s/%s doesn't have a ClusterIP", s.Namespace, s.Name)
		}

		hosts = append(hosts, s.Spec.ClusterIP)
	}

	if len(hosts) < 1 {
		return nil, nil, fmt.Errorf("no services found")
	}

	// TODO: unify logging
	logger, _ := log.NewProduction(log.Config{
		Level: zap.NewAtomicLevelAt(zapcore.InfoLevel),
	})
	scyllaClient, err := scyllaclient.NewClient(
		scyllaclient.DefaultConfig(hosts...),
		logger.Named("scylla_client"),
	)
	if err != nil {
		return nil, nil, err
	}

	return scyllaClient, hosts, nil
}

func getFirstNonSeedNodeName(sc *scyllav1.ScyllaCluster) string {
	return fmt.Sprintf(
		"%s-%s-%s-"+"2",
		sc.Name,
		sc.Spec.Datacenter.Name,
		sc.Spec.Datacenter.Racks[0].Name,
	)
}
