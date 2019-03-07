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

package validating

import (
	"context"
	"github.com/pkg/errors"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

//////////////////////////////////////////
// Generated scaffolding by kubebuilder //
//////////////////////////////////////////

func init() {
	webhookName := "validating-create-update-cluster"
	if HandlerMap[webhookName] == nil {
		HandlerMap[webhookName] = []admission.Handler{}
	}
	HandlerMap[webhookName] = append(HandlerMap[webhookName], &ClusterCreateUpdateHandler{})
}

// ClusterCreateUpdateHandler handles Cluster
type ClusterCreateUpdateHandler struct {
	// Client manipulates objects in K8s
	Client client.Client

	// Decoder decodes objects
	Decoder types.Decoder
}

func (h *ClusterCreateUpdateHandler) validatingClusterFn(ctx context.Context, obj *scyllav1alpha1.Cluster) (bool, string, error) {

	var allowed bool
	var msg string
	oldObj := &scyllav1alpha1.Cluster{}

	err := h.Client.Get(context.TODO(), naming.NamespacedNameForObject(obj), oldObj)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, "failed to get cluster", errors.WithStack(err)
	}

	// First, check the values
	if allowed, msg = checkValues(obj); !allowed {
		return false, msg, nil
	}

	// Then, check the transitions
	if !apierrors.IsNotFound(err) {
		allowed, msg = checkTransitions(oldObj, obj)
	}

	return allowed, msg, nil
}

var _ admission.Handler = &ClusterCreateUpdateHandler{}

// Handle handles admission requests.
func (h *ClusterCreateUpdateHandler) Handle(ctx context.Context, req types.Request) types.Response {
	obj := &scyllav1alpha1.Cluster{}

	err := h.Decoder.Decode(req, obj)
	if err != nil {
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}

	allowed, reason, err := h.validatingClusterFn(ctx, obj)
	if err != nil {
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}
	return admission.ValidationResponse(allowed, reason)
}

var _ inject.Client = &ClusterCreateUpdateHandler{}

// InjectClient injects the client into the ClusterCreateUpdateHandler
func (h *ClusterCreateUpdateHandler) InjectClient(c client.Client) error {
	h.Client = c
	return nil
}

var _ inject.Decoder = &ClusterCreateUpdateHandler{}

// InjectDecoder injects the decoder into the ClusterCreateUpdateHandler
func (h *ClusterCreateUpdateHandler) InjectDecoder(d types.Decoder) error {
	h.Decoder = d
	return nil
}
