// Copyright (c) 2026 ScyllaDB.

package ctrlclient

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
	discoveryv1listers "k8s.io/client-go/listers/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The listers below serve the ScyllaDBCluster controller, whose pure functions take client-go typed listers for the
// objects it mirrors between clusters. Same shape as listers.go: a generic reader and a thin per-kind wrapper.

type serviceLister struct {
	reader[corev1.Service, *corev1.Service]
}

var _ corev1listers.ServiceLister = serviceLister{}

// NewServiceLister returns a ServiceLister reading through c under ctx.
func NewServiceLister(ctx context.Context, c client.Reader) corev1listers.ServiceLister {
	return serviceLister{reader[corev1.Service, *corev1.Service]{ctx: ctx, c: c}}
}

func (l serviceLister) List(selector labels.Selector) ([]*corev1.Service, error) {
	return l.list("", selector)
}

func (l serviceLister) Services(namespace string) corev1listers.ServiceNamespaceLister {
	return serviceNamespaceLister{reader: l.reader, namespace: namespace}
}

type serviceNamespaceLister struct {
	reader[corev1.Service, *corev1.Service]
	namespace string
}

func (l serviceNamespaceLister) List(selector labels.Selector) ([]*corev1.Service, error) {
	return l.list(l.namespace, selector)
}

func (l serviceNamespaceLister) Get(name string) (*corev1.Service, error) {
	return l.get(l.namespace, name)
}

type endpointsLister struct {
	reader[corev1.Endpoints, *corev1.Endpoints]
}

var _ corev1listers.EndpointsLister = endpointsLister{}

// NewEndpointsLister returns a EndpointsLister reading through c under ctx.
func NewEndpointsLister(ctx context.Context, c client.Reader) corev1listers.EndpointsLister {
	return endpointsLister{reader[corev1.Endpoints, *corev1.Endpoints]{ctx: ctx, c: c}}
}

func (l endpointsLister) List(selector labels.Selector) ([]*corev1.Endpoints, error) {
	return l.list("", selector)
}

func (l endpointsLister) Endpoints(namespace string) corev1listers.EndpointsNamespaceLister {
	return endpointsNamespaceLister{reader: l.reader, namespace: namespace}
}

type endpointsNamespaceLister struct {
	reader[corev1.Endpoints, *corev1.Endpoints]
	namespace string
}

func (l endpointsNamespaceLister) List(selector labels.Selector) ([]*corev1.Endpoints, error) {
	return l.list(l.namespace, selector)
}

func (l endpointsNamespaceLister) Get(name string) (*corev1.Endpoints, error) {
	return l.get(l.namespace, name)
}

type configMapLister struct {
	reader[corev1.ConfigMap, *corev1.ConfigMap]
}

var _ corev1listers.ConfigMapLister = configMapLister{}

// NewConfigMapLister returns a ConfigMapLister reading through c under ctx.
func NewConfigMapLister(ctx context.Context, c client.Reader) corev1listers.ConfigMapLister {
	return configMapLister{reader[corev1.ConfigMap, *corev1.ConfigMap]{ctx: ctx, c: c}}
}

func (l configMapLister) List(selector labels.Selector) ([]*corev1.ConfigMap, error) {
	return l.list("", selector)
}

func (l configMapLister) ConfigMaps(namespace string) corev1listers.ConfigMapNamespaceLister {
	return configMapNamespaceLister{reader: l.reader, namespace: namespace}
}

type configMapNamespaceLister struct {
	reader[corev1.ConfigMap, *corev1.ConfigMap]
	namespace string
}

func (l configMapNamespaceLister) List(selector labels.Selector) ([]*corev1.ConfigMap, error) {
	return l.list(l.namespace, selector)
}

func (l configMapNamespaceLister) Get(name string) (*corev1.ConfigMap, error) {
	return l.get(l.namespace, name)
}

type namespaceLister struct {
	reader[corev1.Namespace, *corev1.Namespace]
}

var _ corev1listers.NamespaceLister = namespaceLister{}

// NewNamespaceLister returns a NamespaceLister reading through c under ctx.
func NewNamespaceLister(ctx context.Context, c client.Reader) corev1listers.NamespaceLister {
	return namespaceLister{reader[corev1.Namespace, *corev1.Namespace]{ctx: ctx, c: c}}
}

func (l namespaceLister) List(selector labels.Selector) ([]*corev1.Namespace, error) {
	return l.list("", selector)
}

func (l namespaceLister) Get(name string) (*corev1.Namespace, error) {
	return l.get("", name)
}

type endpointSliceLister struct {
	reader[discoveryv1.EndpointSlice, *discoveryv1.EndpointSlice]
}

var _ discoveryv1listers.EndpointSliceLister = endpointSliceLister{}

// NewEndpointSliceLister returns a EndpointSliceLister reading through c under ctx.
func NewEndpointSliceLister(ctx context.Context, c client.Reader) discoveryv1listers.EndpointSliceLister {
	return endpointSliceLister{reader[discoveryv1.EndpointSlice, *discoveryv1.EndpointSlice]{ctx: ctx, c: c}}
}

func (l endpointSliceLister) List(selector labels.Selector) ([]*discoveryv1.EndpointSlice, error) {
	return l.list("", selector)
}

func (l endpointSliceLister) EndpointSlices(namespace string) discoveryv1listers.EndpointSliceNamespaceLister {
	return endpointSliceNamespaceLister{reader: l.reader, namespace: namespace}
}

type endpointSliceNamespaceLister struct {
	reader[discoveryv1.EndpointSlice, *discoveryv1.EndpointSlice]
	namespace string
}

func (l endpointSliceNamespaceLister) List(selector labels.Selector) ([]*discoveryv1.EndpointSlice, error) {
	return l.list(l.namespace, selector)
}

func (l endpointSliceNamespaceLister) Get(name string) (*discoveryv1.EndpointSlice, error) {
	return l.get(l.namespace, name)
}

type remoteOwnerLister struct {
	reader[scyllav1alpha1.RemoteOwner, *scyllav1alpha1.RemoteOwner]
}

var _ scyllav1alpha1listers.RemoteOwnerLister = remoteOwnerLister{}

// NewRemoteOwnerLister returns a RemoteOwnerLister reading through c under ctx.
func NewRemoteOwnerLister(ctx context.Context, c client.Reader) scyllav1alpha1listers.RemoteOwnerLister {
	return remoteOwnerLister{reader[scyllav1alpha1.RemoteOwner, *scyllav1alpha1.RemoteOwner]{ctx: ctx, c: c}}
}

func (l remoteOwnerLister) List(selector labels.Selector) ([]*scyllav1alpha1.RemoteOwner, error) {
	return l.list("", selector)
}

func (l remoteOwnerLister) RemoteOwners(namespace string) scyllav1alpha1listers.RemoteOwnerNamespaceLister {
	return remoteOwnerNamespaceLister{reader: l.reader, namespace: namespace}
}

type remoteOwnerNamespaceLister struct {
	reader[scyllav1alpha1.RemoteOwner, *scyllav1alpha1.RemoteOwner]
	namespace string
}

func (l remoteOwnerNamespaceLister) List(selector labels.Selector) ([]*scyllav1alpha1.RemoteOwner, error) {
	return l.list(l.namespace, selector)
}

func (l remoteOwnerNamespaceLister) Get(name string) (*scyllav1alpha1.RemoteOwner, error) {
	return l.get(l.namespace, name)
}

type scyllaDBDatacenterLister struct {
	reader[scyllav1alpha1.ScyllaDBDatacenter, *scyllav1alpha1.ScyllaDBDatacenter]
}

var _ scyllav1alpha1listers.ScyllaDBDatacenterLister = scyllaDBDatacenterLister{}

// NewScyllaDBDatacenterLister returns a ScyllaDBDatacenterLister reading through c under ctx.
func NewScyllaDBDatacenterLister(ctx context.Context, c client.Reader) scyllav1alpha1listers.ScyllaDBDatacenterLister {
	return scyllaDBDatacenterLister{reader[scyllav1alpha1.ScyllaDBDatacenter, *scyllav1alpha1.ScyllaDBDatacenter]{ctx: ctx, c: c}}
}

func (l scyllaDBDatacenterLister) List(selector labels.Selector) ([]*scyllav1alpha1.ScyllaDBDatacenter, error) {
	return l.list("", selector)
}

func (l scyllaDBDatacenterLister) ScyllaDBDatacenters(namespace string) scyllav1alpha1listers.ScyllaDBDatacenterNamespaceLister {
	return scyllaDBDatacenterNamespaceLister{reader: l.reader, namespace: namespace}
}

type scyllaDBDatacenterNamespaceLister struct {
	reader[scyllav1alpha1.ScyllaDBDatacenter, *scyllav1alpha1.ScyllaDBDatacenter]
	namespace string
}

func (l scyllaDBDatacenterNamespaceLister) List(selector labels.Selector) ([]*scyllav1alpha1.ScyllaDBDatacenter, error) {
	return l.list(l.namespace, selector)
}

func (l scyllaDBDatacenterNamespaceLister) Get(name string) (*scyllav1alpha1.ScyllaDBDatacenter, error) {
	return l.get(l.namespace, name)
}

type scyllaDBDatacenterNodesStatusReportLister struct {
	reader[scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, *scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport]
}

var _ scyllav1alpha1listers.ScyllaDBDatacenterNodesStatusReportLister = scyllaDBDatacenterNodesStatusReportLister{}

// NewScyllaDBDatacenterNodesStatusReportLister returns a ScyllaDBDatacenterNodesStatusReportLister reading through c under ctx.
func NewScyllaDBDatacenterNodesStatusReportLister(ctx context.Context, c client.Reader) scyllav1alpha1listers.ScyllaDBDatacenterNodesStatusReportLister {
	return scyllaDBDatacenterNodesStatusReportLister{reader[scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, *scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport]{ctx: ctx, c: c}}
}

func (l scyllaDBDatacenterNodesStatusReportLister) List(selector labels.Selector) ([]*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, error) {
	return l.list("", selector)
}

func (l scyllaDBDatacenterNodesStatusReportLister) ScyllaDBDatacenterNodesStatusReports(namespace string) scyllav1alpha1listers.ScyllaDBDatacenterNodesStatusReportNamespaceLister {
	return scyllaDBDatacenterNodesStatusReportNamespaceLister{reader: l.reader, namespace: namespace}
}

type scyllaDBDatacenterNodesStatusReportNamespaceLister struct {
	reader[scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, *scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport]
	namespace string
}

func (l scyllaDBDatacenterNodesStatusReportNamespaceLister) List(selector labels.Selector) ([]*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, error) {
	return l.list(l.namespace, selector)
}

func (l scyllaDBDatacenterNodesStatusReportNamespaceLister) Get(name string) (*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, error) {
	return l.get(l.namespace, name)
}
