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
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster"
	"github.com/scylladb/scylla-operator/pkg/sidecar"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// UseSidecarControllers uses the controllers for the sidecar
func UseSidecarControllers(m manager.Manager, l log.Logger) error {
	controllers := []func(manager.Manager, log.Logger) error{
		sidecar.Add,
	}
	return addToManager(m, controllers, l)
}

// UseSidecarControllers uses the controllers for the operator
func UseOperatorControllers(m manager.Manager, l log.Logger) error {
	controllers := []func(manager.Manager, log.Logger) error{
		cluster.Add,
	}
	return addToManager(m, controllers, l)
}

// AddToManager adds all Controllers to the Manager
func addToManager(m manager.Manager, controllers []func(manager.Manager, log.Logger) error, l log.Logger) error {
	for _, f := range controllers {
		if err := f(m, l); err != nil {
			return err
		}
	}
	return nil
}
