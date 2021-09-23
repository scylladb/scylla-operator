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
	helpers2 "github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/mermaidclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
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
	"k8s.io/klog/v2"
)

func rolloutTimeoutForScyllaCluster(sc *scyllav1.ScyllaCluster) time.Duration {
	return baseRolloutTimout + time.Duration(getMemberCount(sc))*memberRolloutTimeout
}

func getMemberCount(sc *scyllav1.ScyllaCluster) int32 {
	members := int32(0)
	for _, r := range sc.Spec.Datacenter.Racks {
		members += r.Members
	}

	return members
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

func waitForStatefulSetState(ctx context.Context, client appv1client.AppsV1Interface, namespace, name string, conditions ...func(sc *appsv1.StatefulSet) (bool, error)) (*appsv1.StatefulSet, error) {
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
			for _, condition := range conditions {
				done, err := condition(event.Object.(*appsv1.StatefulSet))
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

func waitForPVCState(ctx context.Context, client corev1client.CoreV1Interface, namespace string, name string, condition func(sc *corev1.PersistentVolumeClaim) (bool, error), o waitForStateOptions) (*corev1.PersistentVolumeClaim, error) {
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
			if o.tolerateDelete {
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
	hosts, err := getHosts(ctx, client, sc)
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

	authToken, err := helpers2.GetAgentAuthTokenFromSecret(tokenSecret)
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

func getHosts(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) ([]string, error) {
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

// getManagerClient gets managerClient using IP address. E2E tests shouldn't rely on InCluster DNS.
func getManagerClient(ctx context.Context, client corev1client.CoreV1Interface) (*mermaidclient.Client, error) {
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
