package controllertools

import (
	"context"
	"fmt"

	kubecontroller "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/controller"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type JobControlInterface interface {
	PatchJob(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
}

type RealJobControl struct {
	KubeClient kubernetes.Interface
	Recorder   record.EventRecorder
}

var _ JobControlInterface = &RealJobControl{}

func (r RealJobControl) PatchJob(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
	_, err := r.KubeClient.BatchV1().Jobs(namespace).Patch(ctx, name, pt, data, options)
	return err
}

type JobControllerRefManager struct {
	kubecontroller.BaseControllerRefManager
	ctx           context.Context
	controllerGVK schema.GroupVersionKind
	JobControl    JobControlInterface
}

func NewJobControllerRefManager(
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	canAdopt func() error,
	JobControl JobControlInterface,
) *JobControllerRefManager {
	return &JobControllerRefManager{
		BaseControllerRefManager: kubecontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		ctx:           ctx,
		controllerGVK: controllerGVK,
		JobControl:    JobControl,
	}
}

func (m *JobControllerRefManager) ClaimJobs(jobs []*batchv1.Job) (map[string]*batchv1.Job, error) {
	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptJob(obj.(*batchv1.Job))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseJob(obj.(*batchv1.Job))
	}

	claimedMap := make(map[string]*batchv1.Job, len(jobs))
	var errors []error
	for _, job := range jobs {
		ok, err := m.ClaimObject(job, match, adopt, release)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if ok {
			claimedMap[job.Name] = job
		}
	}
	return claimedMap, utilerrors.NewAggregate(errors)
}

func (m *JobControllerRefManager) AdoptJob(job *batchv1.Job) error {
	err := m.CanAdopt()
	if err != nil {
		return fmt.Errorf("can't adopt Job %v/%v (%v): %v", job.Namespace, job.Name, job.UID, err)
	}

	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	patchBytes, err := kubecontroller.OwnerRefControllerPatch(m.Controller, m.controllerGVK, job.UID)
	if err != nil {
		return err
	}

	return m.JobControl.PatchJob(m.ctx, job.Namespace, job.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
}

func (m *JobControllerRefManager) ReleaseJob(job *batchv1.Job) error {
	klog.V(2).InfoS("Patching Job to remove its controllerRef",
		"Job",
		klog.KObj(job),
		"ControllerRef",
		fmt.Sprintf("%s/%s:%s", m.controllerGVK.GroupVersion(), m.controllerGVK.Kind, m.Controller.GetName()),
	)

	patchBytes, err := kubecontroller.DeleteOwnerRefStrategicMergePatch(job.UID, m.Controller.GetUID())
	if err != nil {
		return err
	}

	err = m.JobControl.PatchJob(m.ctx, job.Namespace, job.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// If the Job no longer exists, ignore it.
			klog.V(4).InfoS("Couldn't patch Job as it was missing", klog.KObj(job))
			return nil
		}

		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases:
			// 1. the Job has no owner reference
			// 2. the UID of the Job doesn't match because it was recreated
			// In both cases, the error can be ignored.
			klog.V(4).InfoS("Couldn't patch Job as it was invalid", klog.KObj(job))
			return nil
		}

		return err
	}

	return nil
}
