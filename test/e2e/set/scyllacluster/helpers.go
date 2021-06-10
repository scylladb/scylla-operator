package scyllacluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/helpers"
	"github.com/scylladb/scylla-operator/pkg/mermaidclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
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

func syncTimeoutForScyllaCluster(sc *scyllav1.ScyllaCluster) time.Duration {
	tasks := int64(len(sc.Spec.Repairs) + len(sc.Spec.Backups))
	return baseManagerSyncTimeout + time.Duration(tasks)*managerTaskSyncTimeout
}

func contextForManagerSync(parent context.Context, sc *scyllav1.ScyllaCluster) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, syncTimeoutForScyllaCluster(sc))
}

func scyllaClusterRolledOut(sc *scyllav1.ScyllaCluster) (bool, error) {
	// TODO: check observed generation when it's added.
	// TODO: this should be more straight forward - we need better status (conditions, aggregated state, ...)

	for _, r := range sc.Spec.Datacenter.Racks {
		if sc.Status.Racks[r.Name].Members != r.Members {
			return false, nil
		}

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

func waitForScyllaClusterState(ctx context.Context, client scyllav1client.ScyllaV1Interface, namespace string, name string, conditions ...func(sc *scyllav1.ScyllaCluster) (bool, error)) (*scyllav1.ScyllaCluster, error) {
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
			for _, condition := range conditions {
				done, err := condition(event.Object.(*scyllav1.ScyllaCluster))
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
	return event.Object.(*scyllav1.ScyllaCluster), nil
}

func waitForStatefulSetRollout(ctx context.Context, client appv1client.AppsV1Interface, namespace, name string) (*appsv1.StatefulSet, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.StatefulSets(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.StatefulSets(namespace).Watch(ctx, options)
		},
	}
	event, err := watchtools.UntilWithSync(ctx, lw, &appsv1.StatefulSet{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			sts := event.Object.(*appsv1.StatefulSet)
			return sts.Status.ObservedGeneration >= sts.Generation &&
				sts.Status.CurrentRevision == sts.Status.UpdateRevision &&
				sts.Status.ReadyReplicas == *sts.Spec.Replicas, nil
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*appsv1.StatefulSet), nil
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

	authToken, err := helpers.GetAgentAuthToken(ctx, client, sc.Name, sc.Namespace)
	if err != nil {
		return nil, nil, fmt.Errorf("get auth token: %w", err)
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

func getManagerClient(ctx context.Context, client corev1client.CoreV1Interface) (*mermaidclient.Client, error) {
	managerService, err := client.Services(managerNamespace).Get(ctx, "scylla-manager", metav1.GetOptions{})
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

func getFirstNonSeedNodeName(sc *scyllav1.ScyllaCluster) string {
	return getNodeName(sc, 2)
}

func getNodeName(sc *scyllav1.ScyllaCluster, idx int) string {
	return fmt.Sprintf(
		"%s-%s-%s-%d",
		sc.Name,
		sc.Spec.Datacenter.Name,
		sc.Spec.Datacenter.Racks[0].Name,
		idx,
	)
}

func restartRack(ctx context.Context, client appv1client.AppsV1Interface, r scyllav1.RackSpec, sc *scyllav1.ScyllaCluster) error {
	sts, err := client.StatefulSets(sc.Namespace).Get(ctx, naming.StatefulSetNameForRack(r, sc), metav1.GetOptions{})
	if err != nil {
		return err
	}

	before, err := runtime.Encode(scheme.DefaultJSONEncoder(), sts)
	if err != nil {
		return err
	}

	if sts.Spec.Template.ObjectMeta.Annotations == nil {
		sts.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	sts.Spec.Template.ObjectMeta.Annotations["e2e.scylladb.com/restartedAt"] = time.Now().Format(time.RFC3339)

	after, err := runtime.Encode(scheme.DefaultJSONEncoder(), sts)
	if err != nil {
		return err
	}
	patch, err := strategicpatch.CreateTwoWayMergePatch(before, after, sts)
	if err != nil {
		return err
	}

	_, err = client.StatefulSets(sc.Namespace).Patch(ctx, sts.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return err
	}

	return nil
}
