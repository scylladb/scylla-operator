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

package controller

import (
	"github.com/scylladb/scylla-operator/pkg/controller/cluster"
	"github.com/scylladb/scylla-operator/pkg/sidecar"
	"github.com/scylladb/scylla-operator/pkg/webhook/default_server"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// UseSidecarControllers uses the controllers for the sidecar
func UseSidecarControllers(m manager.Manager) error {
	controllers := []func(manager.Manager) error{
		sidecar.Add,
	}
	return addToManager(m, controllers)
}

// UseSidecarControllers uses the controllers for the operator
func UseOperatorControllers(m manager.Manager) error {
	controllers := []func(manager.Manager) error{
		cluster.Add,
		defaultserver.Add,
	}
	return addToManager(m, controllers)
}

// AddToManager adds all Controllers to the Manager
func addToManager(m manager.Manager, controllers []func(manager.Manager) error) error {
	for _, f := range controllers {
		if err := f(m); err != nil {
			return err
		}
	}
	return nil
}
