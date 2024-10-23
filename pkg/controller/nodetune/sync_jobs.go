// Copyright (C) 2021 ScyllaDB

package nodetune

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/c9s/goprocinfo/linux"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/semver"
	"github.com/scylladb/scylla-operator/pkg/util/cloud"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	"github.com/scylladb/scylla-operator/pkg/util/network"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (ncdc *Controller) makeJobsForNode(ctx context.Context) ([]*batchv1.Job, error) {
	pod, err := ncdc.selfPodLister.Pods(ncdc.namespace).Get(ncdc.podName)
	if err != nil {
		return nil, fmt.Errorf("can't get self Pod %q: %w", naming.ManualRef(ncdc.namespace, ncdc.podName), err)
	}

	var jobs []*batchv1.Job

	cr, err := ncdc.newOwningDSControllerRef()
	if err != nil {
		return nil, fmt.Errorf("can't get controller ref: %w", err)
	}

	jobs = append(jobs, makePerftuneJobForNode(
		cr,
		ncdc.namespace,
		ncdc.nodeConfigName,
		ncdc.nodeName,
		ncdc.nodeUID,
		ncdc.scyllaImage,
		&pod.Spec,
	))

	return jobs, nil
}

func (ncdc *Controller) makePerftuneJobForContainers(ctx context.Context, podSpec *corev1.PodSpec, optimizablePods []*corev1.Pod, scyllaContainerIDs []string) (*batchv1.Job, error) {
	if len(optimizablePods) == 0 {
		klog.V(2).InfoS("No optimizable pod found on this node")
		return nil, nil
	}

	cpuInfo, err := linux.ReadCPUInfo("/proc/cpuinfo")
	if err != nil {
		return nil, fmt.Errorf("can't parse cpuinfo from %q: %w", "/proc/cpuinfo", err)
	}

	hostFullCpuset, err := cpuset.Parse(fmt.Sprintf("0-%d", cpuInfo.NumCPU()-1))
	if err != nil {
		return nil, fmt.Errorf("can't parse full mask: %w", err)
	}

	irqCPUs, err := getIRQCPUs(ctx, ncdc.kubeletPodResourcesClient, optimizablePods, hostFullCpuset)
	if err != nil {
		return nil, fmt.Errorf("can't get IRQ CPUs: %w", err)
	}

	rawDataHostPaths, err := scyllaDataDirMountHostPaths(ctx, ncdc.criClient, optimizablePods)
	if err != nil {
		return nil, fmt.Errorf("can't find data dir host path: %w", err)
	}

	if len(rawDataHostPaths) == 0 {
		return nil, fmt.Errorf("no data mount host path found")
	}

	disableWritebackCache := false
	if cloud.OnGKE() {
		scyllaVersion, err := naming.ImageToVersion(ncdc.scyllaImage)
		if err != nil {
			return nil, fmt.Errorf("can't determine scylla image version %q: %w", ncdc.scyllaImage, err)
		}
		sv := semver.NewScyllaVersion(scyllaVersion)

		if sv.SupportFeatureSafe(semver.ScyllaVersionThatSupportsDisablingWritebackCache) {
			disableWritebackCache = true
		}
	}

	// Because perftune is not running in chroot we need to resolve any absolute symlinks in these paths.
	dataHostPaths := make([]string, 0, len(rawDataHostPaths))
	for _, rp := range rawDataHostPaths {
		p, err := filepath.EvalSymlinks(rp)
		if err != nil {
			return nil, fmt.Errorf("can't resolve symlink %q: %w", rp, err)
		}

		dataHostPaths = append(dataHostPaths, p)
	}

	// Sort paths to have stable representation for the same set host paths.
	sort.Strings(dataHostPaths)

	cr, err := ncdc.newOwningDSControllerRef()
	if err != nil {
		return nil, fmt.Errorf("can't get controller ref: %w", err)
	}

	ifaces, err := network.FindEthernetInterfaces()
	if err != nil {
		return nil, fmt.Errorf("can't find local interface")
	}
	ifaceNames := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		ifaceNames = append(ifaceNames, iface.Name)
	}

	// Sort interface names to have stable representation for the same set of interfaces.
	sort.Strings(ifaceNames)

	klog.V(4).Info("Tuning network interfaces", "ifaces", ifaceNames)

	return makePerftuneJobForContainers(
		cr,
		ncdc.namespace,
		ncdc.nodeConfigName,
		ncdc.nodeName,
		ncdc.nodeUID,
		ncdc.scyllaImage,
		irqCPUs.FormatMask(),
		dataHostPaths,
		disableWritebackCache,
		podSpec,
		ifaceNames,
		scyllaContainerIDs,
	)
}

func (ncdc *Controller) makeResourceLimitJobsForContainers(ctx context.Context, podSpec *corev1.PodSpec, scyllaPods []*corev1.Pod) ([]*batchv1.Job, error) {
	cr, err := ncdc.newOwningDSControllerRef()
	if err != nil {
		return nil, fmt.Errorf("can't get controller ref: %w", err)
	}

	rlimitsJobs := make([]*batchv1.Job, 0, len(scyllaPods))

	for _, scyllaPod := range scyllaPods {
		containerID, err := getScyllaContainerIDInCRIFormat(scyllaPod)
		if err != nil {
			return nil, fmt.Errorf("can't get scylla containerID in CRI format: %w", err)
		}

		containerStatus, err := ncdc.criClient.Inspect(ctx, containerID)
		if err != nil {
			return nil, fmt.Errorf("can't inspect Scylla Pod %q container %q: %w", naming.ObjRef(scyllaPod), containerID, err)
		}

		if containerStatus.Info.PID == nil {
			klog.Errorf("can't determine Scylla container process PIDs from CRI, their resource process limits won't be raised")
			continue
		}

		rlimitsJob, err := makeRlimitsJobForContainer(cr, ncdc.namespace, ncdc.nodeConfigName, ncdc.nodeName, ncdc.nodeUID, ncdc.operatorImage, podSpec, scyllaPod, *containerStatus.Info.PID)
		if err != nil {
			return nil, fmt.Errorf("can't make rlimits jobs: %w", err)
		}

		rlimitsJobs = append(rlimitsJobs, rlimitsJob)
	}

	return rlimitsJobs, nil
}

func (ncdc *Controller) makeJobsForContainers(ctx context.Context) ([]*batchv1.Job, error) {
	localScyllaPods, err := ncdc.localScyllaPodsLister.List(naming.ScyllaSelector())
	if err != nil {
		return nil, fmt.Errorf("can't list local scylla pods: %w", err)
	}

	localScyllaPods = slices.FilterOut(localScyllaPods, func(pod *corev1.Pod) bool {
		return pod.DeletionTimestamp != nil
	})

	// Container Jobs are created based on Pods' fields.
	// Pods are sorted to make sure the generated Job specs are consistent across reconciliations for an equal set of Pods.
	sort.Slice(localScyllaPods, func(i, j int) bool {
		return localScyllaPods[i].Name < localScyllaPods[j].Name
	})

	var optimizablePods []*corev1.Pod
	var runningScyllaPods []*corev1.Pod
	var optimizableScyllaContainerIDs []string

	for _, scyllaPod := range localScyllaPods {
		if !controllerhelpers.IsScyllaContainerRunning(scyllaPod) {
			klog.V(4).Infof("Pod %q is a candidate for optimizations but scylla container isn't running yet", naming.ObjRef(scyllaPod))
			continue
		}

		containerID, err := controllerhelpers.GetScyllaContainerID(scyllaPod)
		if err != nil || len(containerID) == 0 {
			ncdc.eventRecorder.Event(ncdc.newNodeConfigObjectRef(), corev1.EventTypeWarning, "MissingContainerID", "Scylla container status is missing a containerID. Scylla won't wait for tuning to finish.")
			continue
		}

		runningScyllaPods = append(runningScyllaPods, scyllaPod)

		if !controllerhelpers.IsPodTunable(scyllaPod) {
			klog.V(4).Infof("Pod %q isn't a subject for optimizations", naming.ObjRef(scyllaPod))
			continue
		}

		klog.V(4).Infof("Pod %s is subject for optimizations", naming.ObjRef(scyllaPod))
		optimizablePods = append(optimizablePods, scyllaPod)
		optimizableScyllaContainerIDs = append(optimizableScyllaContainerIDs, containerID)
	}

	selfPod, err := ncdc.selfPodLister.Pods(ncdc.namespace).Get(ncdc.podName)
	if err != nil {
		return nil, fmt.Errorf("can't get Pod %q: %w", naming.ManualRef(ncdc.namespace, ncdc.podName), err)
	}

	perftuneJob, err := ncdc.makePerftuneJobForContainers(ctx, &selfPod.Spec, optimizablePods, optimizableScyllaContainerIDs)
	if err != nil {
		return nil, fmt.Errorf("can't make perftune jobs: %w", err)
	}

	resourceLimitsJobs, err := ncdc.makeResourceLimitJobsForContainers(ctx, &selfPod.Spec, runningScyllaPods)
	if err != nil {
		return nil, fmt.Errorf("can't make resource limits jobs: %w", err)
	}

	var containerJobs []*batchv1.Job
	if perftuneJob != nil {
		containerJobs = append(containerJobs, perftuneJob)
	}
	if resourceLimitsJobs != nil {
		containerJobs = append(containerJobs, resourceLimitsJobs...)
	}

	return containerJobs, nil
}

func (ncdc *Controller) syncJobs(ctx context.Context, jobs map[string]*batchv1.Job, nodeStatus *scyllav1alpha1.NodeConfigNodeStatus) error {
	requiredForNode, err := ncdc.makeJobsForNode(ctx)
	if err != nil {
		return fmt.Errorf("can't make Jobs for node: %w", err)
	}

	requiredForContainers, err := ncdc.makeJobsForContainers(ctx)
	if err != nil {
		return fmt.Errorf("can't make Jobs for containers: %w", err)
	}

	var requiredJobs []*batchv1.Job
	requiredJobs = append(requiredJobs, requiredForNode...)
	if requiredForContainers != nil {
		requiredJobs = append(requiredJobs, requiredForContainers...)
	}

	err = ncdc.pruneJobs(ctx, jobs, requiredJobs)
	if err != nil {
		return fmt.Errorf("can't prune Jobs: %w", err)
	}

	finished := true

	containerRequiredJobTypes := make(map[string][]naming.NodeConfigJobType)
	containerCompletedJobTypes := make(map[string][]naming.NodeConfigJobType)

	klog.V(4).InfoS("Required jobs", "Count", len(requiredJobs))
	for _, j := range requiredJobs {
		fresh, _, err := resourceapply.ApplyJob(ctx, ncdc.kubeClient.BatchV1(), ncdc.namespacedJobLister, ncdc.eventRecorder, j, resourceapply.ApplyOptions{})
		if err != nil {
			return fmt.Errorf("can't create job %s: %w", naming.ObjRef(j), err)
		}

		t, found := j.Labels[naming.NodeConfigJobTypeLabel]
		if !found {
			return fmt.Errorf("job %q is missing %q label", naming.ObjRef(j), naming.NodeConfigJobTypeLabel)
		}

		switch jobType := naming.NodeConfigJobType(t); jobType {
		case naming.NodeConfigJobTypeNode:
			// FIXME: Extract into a function and double check how jobs report status.
			if fresh.Status.CompletionTime == nil {
				klog.V(4).InfoS("Job isn't completed yet", "Job", klog.KObj(fresh))
				finished = false
				break
			}
			klog.V(4).InfoS("Job is completed", "Job", klog.KObj(fresh))

		case naming.NodeConfigJobTypeContainerPerftune, naming.NodeConfigJobTypeContainerResourceLimits:
			// We have successfully applied the job definition so the data should always be present at this point.
			nodeConfigJobDataString, found := fresh.Annotations[naming.NodeConfigJobData]
			if !found {
				return fmt.Errorf("internal error: job %q is missing %q annotation", klog.KObj(fresh), naming.NodeConfigJobData)
			}

			jobData := &containerJobData{}
			err = json.Unmarshal([]byte(nodeConfigJobDataString), jobData)
			if err != nil {
				return fmt.Errorf("internal error: can't unmarshal node config data for job %q: %w", klog.KObj(fresh), err)
			}

			for _, containerID := range jobData.ContainerIDs {
				containerRequiredJobTypes[containerID] = append(containerRequiredJobTypes[containerID], jobType)
				if fresh.Status.CompletionTime != nil {
					containerCompletedJobTypes[containerID] = append(containerCompletedJobTypes[containerID], jobType)
				}
			}

			if fresh.Status.CompletionTime == nil {
				klog.V(4).InfoS("Job isn't completed yet", "Job", klog.KObj(fresh))
				break
			}
		default:
			return fmt.Errorf("job %q has an unkown type %q", naming.ObjRef(j), t)
		}
	}

	nodeStatus.TunedNode = finished
	for containerID, requiredContainerJobTypes := range containerRequiredJobTypes {
		if equality.Semantic.DeepEqual(containerCompletedJobTypes[containerID], requiredContainerJobTypes) {
			nodeStatus.TunedContainers = append(nodeStatus.TunedContainers, containerID)
		}
	}
	sort.Strings(nodeStatus.TunedContainers)

	return nil
}

func (ncdc *Controller) pruneJobs(ctx context.Context, jobs map[string]*batchv1.Job, requiredJobs []*batchv1.Job) error {
	var errs []error
	for _, j := range jobs {
		if j.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range requiredJobs {
			if j.Name == req.Name && j.Namespace == req.Namespace {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		klog.InfoS("Removing stale Job", "Job", klog.KObj(j))
		propagationPolicy := metav1.DeletePropagationBackground
		err := ncdc.kubeClient.BatchV1().Jobs(j.Namespace).Delete(ctx, j.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &j.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return utilerrors.NewAggregate(errs)
}
