// Copyright (C) 2025 ScyllaDB

package statusreport

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

const (
	localhost = "localhost"
)

type Controller struct {
	*controllertools.Observer

	namespace string
	podName   string

	kubeClient kubernetes.Interface
	podLister  corev1listers.PodLister
}

func NewController(
	namespace string,
	podName string,
	kubeClient kubernetes.Interface,
	podInformer corev1informers.PodInformer,
) (*Controller, error) {
	c := &Controller{
		namespace: namespace,
		podName:   podName,

		kubeClient: kubeClient,
		podLister:  podInformer.Lister(),
	}

	observer := controllertools.NewObserver(
		"status-report",
		kubeClient.CoreV1().Events(corev1.NamespaceAll),
		c.Sync,
	)

	podHandler, err := podInformer.Informer().AddEventHandler(observer.GetGenericHandlers())
	if err != nil {
		return nil, fmt.Errorf("can't add event handler to Pod informer: %w", err)
	}
	observer.AddCachesToSync(podHandler.HasSynced)

	c.Observer = observer

	return c, nil
}

func (c *Controller) Sync(ctx context.Context) error {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing observer", "Name", c.Observer.Name(), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing observer", "Name", c.Observer.Name(), "duration", time.Since(startTime))
	}()

	pod, err := c.podLister.Pods(c.namespace).Get(c.podName)
	if err != nil {
		return fmt.Errorf("can't get Pod %q: %v", naming.ManualRef(c.namespace, c.podName), err)
	}

	nodeStatusReport := getNodeStatusReport(ctx)
	encodedNodeStatusReport, err := nodeStatusReport.Encode()
	if err != nil {
		return fmt.Errorf("can't encode node status report: %w", err)
	}

	encodedNodeStatusReportString := string(encodedNodeStatusReport)

	if controllerhelpers.HasMatchingAnnotation(pod, naming.NodeStatusReportAnnotation, encodedNodeStatusReportString) {
		klog.V(5).InfoS("Pod already has up-to-date node status report annotation", "Pod", naming.ObjRef(pod))
		return nil
	}

	klog.V(4).InfoS("Patching Pod with new node status report annotation", "Pod", naming.ObjRef(pod), "NodeStatusReport", nodeStatusReport)
	patch, err := controllerhelpers.PrepareSetAnnotationPatch(pod, naming.NodeStatusReportAnnotation, pointer.Ptr[string](string(encodedNodeStatusReport)))
	if err != nil {
		return fmt.Errorf("can't prepare annotation patch: %w", err)
	}

	_, err = c.kubeClient.CoreV1().Pods(c.namespace).Patch(ctx, c.podName, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("can't patch pod %q: %w", naming.ObjRef(pod), err)
	}

	klog.V(4).InfoS("Finished patching Pod with new node status report annotation", "Pod", naming.ObjRef(pod))

	return nil
}

func getNodeStatusReport(ctx context.Context) *internalapi.NodeStatusReport {
	scyllaClient, err := controllerhelpers.NewScyllaClientForLocalhost(corev1.IPv4Protocol)
	if err != nil {
		return &internalapi.NodeStatusReport{
			Error: pointer.Ptr(fmt.Errorf("can't create Scylla client for localhost: %w", err).Error()),
		}
	}
	defer scyllaClient.Close()

	nodeStatuses, err := scyllaClient.NodesStatusInfo(ctx, localhost)
	if err != nil {
		return &internalapi.NodeStatusReport{
			Error: pointer.Ptr(fmt.Errorf("can't get node status info: %w", err).Error()),
		}
	}

	observedNodeStatuses := make([]scyllav1alpha1.ObservedNodeStatus, 0, len(nodeStatuses))
	for _, ns := range nodeStatuses {
		observedNodeStatuses = append(observedNodeStatuses, scyllav1alpha1.ObservedNodeStatus{
			HostID: ns.HostID,
			Status: scyllaClientNodeStatusToScyllaV1Alpha1NodeStatus(ns.Status),
		})
	}

	return &internalapi.NodeStatusReport{
		ObservedNodes: observedNodeStatuses,
	}
}

func scyllaClientNodeStatusToScyllaV1Alpha1NodeStatus(status scyllaclient.NodeStatus) scyllav1alpha1.NodeStatus {
	switch status {
	case scyllaclient.NodeStatusUp:
		return scyllav1alpha1.NodeStatusUp

	default:
		return scyllav1alpha1.NodeStatusDown

	}
}
