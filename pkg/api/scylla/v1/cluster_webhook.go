/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var clusterlog = logf.Log.WithName("cluster-resource")

func (r *ScyllaCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/mutate-scylla-scylladb-com-v1-scyllacluster,mutating=true,failurePolicy=fail,groups=scylla.scylladb.com,resources=scyllaclusters,versions=v1,name=webhook.scylla.scylladb.com
// +kubebuilder:webhook:verbs=create;update,path=/validate-scylla-scylladb-com-v1-scyllacluster,mutating=false,failurePolicy=fail,groups=scylla.scylladb.com,resources=scyllaclusters,versions=v1,name=webhook.scylla.scylladb.com

var _ webhook.Defaulter = &ScyllaCluster{}
var _ webhook.Validator = &ScyllaCluster{}

const (
	DefaultGenericUpgradePollInterval = time.Second
)

func (c *ScyllaCluster) Default() {
	for _, r := range c.Spec.Datacenter.Racks {
		// Empty agent resources
		if reflect.DeepEqual(r.AgentResources, corev1.ResourceRequirements{}) {
			r.AgentResources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("10M"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("200M"),
				},
			}
		}
	}

	for i, repairTask := range c.Spec.Repairs {
		if repairTask.StartDate == nil {
			c.Spec.Repairs[i].StartDate = pointer.StringPtr("now")
		}
		if repairTask.Interval == nil {
			c.Spec.Repairs[i].Interval = pointer.StringPtr("0")
		}
		if repairTask.NumRetries == nil {
			c.Spec.Repairs[i].NumRetries = pointer.Int64Ptr(3)
		}
		if repairTask.SmallTableThreshold == nil {
			c.Spec.Repairs[i].SmallTableThreshold = pointer.StringPtr("1GiB")
		}
		if repairTask.Intensity == nil {
			c.Spec.Repairs[i].Intensity = pointer.StringPtr("1")
		}
		if repairTask.Parallel == nil {
			c.Spec.Repairs[i].Parallel = pointer.Int64Ptr(0)
		}
	}

	for i, backupTask := range c.Spec.Backups {
		if backupTask.StartDate == nil {
			c.Spec.Backups[i].StartDate = pointer.StringPtr("now")
		}
		if backupTask.Interval == nil {
			c.Spec.Backups[i].Interval = pointer.StringPtr("0")
		}
		if backupTask.NumRetries == nil {
			c.Spec.Backups[i].NumRetries = pointer.Int64Ptr(3)
		}
		if backupTask.Retention == nil {
			c.Spec.Backups[i].Retention = pointer.Int64Ptr(3)
		}
	}

	if c.Spec.GenericUpgrade == nil {
		c.Spec.GenericUpgrade = &GenericUpgradeSpec{}
	}

	if c.Spec.GenericUpgrade.FailureStrategy == "" {
		c.Spec.GenericUpgrade.FailureStrategy = GenericUpgradeFailureStrategyRetry
	}

	if c.Spec.GenericUpgrade.PollInterval == nil {
		c.Spec.GenericUpgrade.PollInterval = &metav1.Duration{Duration: DefaultGenericUpgradePollInterval}
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ScyllaCluster) ValidateCreate() error {
	clusterlog.Info("validate create", "name", r.Name)

	// First, check the values
	if err := checkValues(r); err != nil {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ScyllaCluster) ValidateUpdate(old runtime.Object) error {
	clusterlog.Info("validate update", "name", r.Name)

	// First, check the values
	if err := checkValues(r); err != nil {
		return err
	}

	if err := checkTransitions(old.(*ScyllaCluster), r); err != nil {
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ScyllaCluster) ValidateDelete() error {
	clusterlog.Info("validate delete", "name", r.Name)
	// no validation during delete
	return nil
}
