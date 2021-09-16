// Copyright (C) 2021 ScyllaDB

package nodeconfigpod

import (
	"context"
	"fmt"
	"path"

	"github.com/c9s/goprocinfo/linux"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/helpers"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllanodeconfig/resource"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/semver"
	"github.com/scylladb/scylla-operator/pkg/util/cloud"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	"github.com/scylladb/scylla-operator/pkg/util/network"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (c *Controller) syncJobs(ctx context.Context, snt *scyllav1alpha1.ScyllaNodeConfig, scyllaPods []*corev1.Pod, jobs map[string]*batchv1.Job) error {
	requiredJobs, err := c.makeJobs(ctx, snt, scyllaPods, jobs)
	if err != nil {
		return fmt.Errorf("can't make Jobs: %w", err)
	}
	err = c.pruneJobs(ctx, jobs, requiredJobs)
	if err != nil {
		return fmt.Errorf("can't delete Jobs: %w", err)
	}

	for _, j := range requiredJobs {
		_, _, err := resourceapply.ApplyJob(ctx, c.kubeClient.BatchV1(), c.jobLister, c.eventRecorder, j)
		if err != nil {
			return fmt.Errorf("can't create job %s: %w", naming.ObjRef(j), err)
		}
	}
	return nil
}

func (c *Controller) makeJobs(ctx context.Context, snt *scyllav1alpha1.ScyllaNodeConfig, scyllaPods []*corev1.Pod, existingJobs map[string]*batchv1.Job) ([]*batchv1.Job, error) {
	var jobs []*batchv1.Job

	var optimizedPods []*corev1.Pod
	for i, scyllaPod := range scyllaPods {
		if scyllaPod.Status.QOSClass == corev1.PodQOSGuaranteed {
			klog.V(4).Infof("Pod %s is subject for optimizations", naming.ObjRef(scyllaPod))
			optimizedPods = append(optimizedPods, scyllaPods[i])
		} else {
			klog.V(4).Infof("Pod %s is not subject for optimizations", naming.ObjRef(scyllaPod))
		}
	}

	if snt.Spec.DisableOptimizations || len(optimizedPods) == 0 {
		klog.V(4).Infof("Nothing todo")
		return jobs, nil
	}

	for _, pod := range optimizedPods {
		if !helpers.IsScyllaContainerRunning(pod) {
			klog.V(4).Infof("Not all Scylla Pods are running, will retry in a bit")
			if j, ok := existingJobs[naming.PerftuneJobName(c.nodeName)]; ok {
				jobs = append(jobs, j)
			}
			return jobs, nil
		}
	}

	cpuInfo, err := linux.ReadCPUInfo(path.Join(naming.HostFilesystemDirName, "/proc/cpuinfo"))
	if err != nil {
		return nil, fmt.Errorf("couldn't parse cpuinfo from %q: %w", "/proc/cpuinfo", err)
	}

	hostFullCpuset, err := cpuset.Parse(fmt.Sprintf("0-%d", cpuInfo.NumCPU()-1))
	if err != nil {
		return nil, fmt.Errorf("can't parse full mask: %w", err)
	}

	irqCPUs, err := getIRQCPUs(ctx, c.criClient, optimizedPods, hostFullCpuset)
	if err != nil {
		return nil, fmt.Errorf("can't get IRQ CPUs: %w", err)
	}

	dataHostPaths, err := scyllaDataDirHostPaths(ctx, c.criClient, optimizedPods)
	if err != nil {
		return nil, fmt.Errorf("can't find data dir host path: %w", err)
	}

	iface, err := network.FindEthernetInterface()
	if err != nil {
		return nil, fmt.Errorf("can't find local interface")
	}

	disableWritebackCache := false

	if cloud.OnGKE() {
		scyllaVersion, err := naming.ImageToVersion(c.scyllaImage)
		if err != nil {
			return nil, fmt.Errorf("can't determine scylla image version %q: %w", c.scyllaImage, err)
		}
		sv := semver.NewScyllaVersion(scyllaVersion)

		if sv.SupportFeatureSafe(semver.ScyllaVersionThatSupportsDisablingWritebackCache) {
			disableWritebackCache = true
		}
	}

	jobs = append(jobs, resource.PerftuneJob(snt, c.nodeName, c.scyllaImage, iface.Name, irqCPUs.FormatMask(), dataHostPaths, disableWritebackCache))

	return jobs, nil
}

func (c *Controller) pruneJobs(ctx context.Context, jobs map[string]*batchv1.Job, requiredJobs []*batchv1.Job) error {
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
		err := c.kubeClient.BatchV1().Jobs(j.Namespace).Delete(ctx, j.Name, metav1.DeleteOptions{
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
