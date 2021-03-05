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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
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

// +kubebuilder:webhook:verbs=create;update,path=/validate-scylla-scylladb-com-v1-scyllacluster,mutating=false,failurePolicy=fail,groups=scylla.scylladb.com,resources=scyllaclusters,versions=v1,name=webhook.scylla.scylladb.com,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &ScyllaCluster{}

const (
	DefaultGenericUpgradePollInterval = time.Second
)

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
