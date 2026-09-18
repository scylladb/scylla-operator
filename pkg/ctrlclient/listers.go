// Copyright (c) 2026 ScyllaDB.

package ctrlclient

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The pure functions of the controllers take client-go typed listers for the kinds they look up by name. The listers
// below implement those interfaces over a controller-runtime client, bound to the context of one reconciliation, so
// that the functions keep their signatures and read through the same (read-your-writes) client as the rest of the
// controller. Only the kinds the pure functions take are covered; add the next one here when it is needed.

type reader[T any, PT Object[T]] struct {
	ctx context.Context
	c   client.Reader
}

func (r reader[T, PT]) get(namespace, name string) (PT, error) {
	return Get[T, PT](r.ctx, r.c, namespace, name)
}

func (r reader[T, PT]) list(namespace string, selector labels.Selector) ([]PT, error) {
	return List[T, PT](r.ctx, r.c, namespace, selector)
}

type podLister struct {
	reader[corev1.Pod, *corev1.Pod]
}

var _ corev1listers.PodLister = podLister{}

// NewPodLister returns a PodLister reading through c under ctx.
func NewPodLister(ctx context.Context, c client.Reader) corev1listers.PodLister {
	return podLister{reader[corev1.Pod, *corev1.Pod]{ctx: ctx, c: c}}
}

func (l podLister) List(selector labels.Selector) ([]*corev1.Pod, error) {
	return l.list("", selector)
}

func (l podLister) Pods(namespace string) corev1listers.PodNamespaceLister {
	return podNamespaceLister{reader: l.reader, namespace: namespace}
}

type podNamespaceLister struct {
	reader[corev1.Pod, *corev1.Pod]
	namespace string
}

func (l podNamespaceLister) List(selector labels.Selector) ([]*corev1.Pod, error) {
	return l.list(l.namespace, selector)
}

func (l podNamespaceLister) Get(name string) (*corev1.Pod, error) {
	return l.get(l.namespace, name)
}

type secretLister struct {
	reader[corev1.Secret, *corev1.Secret]
}

var _ corev1listers.SecretLister = secretLister{}

// NewSecretLister returns a SecretLister reading through c under ctx.
func NewSecretLister(ctx context.Context, c client.Reader) corev1listers.SecretLister {
	return secretLister{reader[corev1.Secret, *corev1.Secret]{ctx: ctx, c: c}}
}

func (l secretLister) List(selector labels.Selector) ([]*corev1.Secret, error) {
	return l.list("", selector)
}

func (l secretLister) Secrets(namespace string) corev1listers.SecretNamespaceLister {
	return secretNamespaceLister{reader: l.reader, namespace: namespace}
}

type secretNamespaceLister struct {
	reader[corev1.Secret, *corev1.Secret]
	namespace string
}

func (l secretNamespaceLister) List(selector labels.Selector) ([]*corev1.Secret, error) {
	return l.list(l.namespace, selector)
}

func (l secretNamespaceLister) Get(name string) (*corev1.Secret, error) {
	return l.get(l.namespace, name)
}
