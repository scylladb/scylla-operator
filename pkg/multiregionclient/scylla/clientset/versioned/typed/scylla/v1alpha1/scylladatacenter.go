// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scheme "github.com/scylladb/scylla-operator/pkg/multiregionclient/scylla/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ScyllaDatacentersGetter has a method to return a ScyllaDatacenterInterface.
// A group's client should implement this interface.
type ScyllaDatacentersGetter interface {
	ScyllaDatacenters(namespace string) ScyllaDatacenterInterface
}

// ScyllaDatacenterInterface has methods to work with ScyllaDatacenter resources.
type ScyllaDatacenterInterface interface {
	Create(ctx context.Context, scyllaDatacenter *v1alpha1.ScyllaDatacenter, opts v1.CreateOptions) (*v1alpha1.ScyllaDatacenter, error)
	Update(ctx context.Context, scyllaDatacenter *v1alpha1.ScyllaDatacenter, opts v1.UpdateOptions) (*v1alpha1.ScyllaDatacenter, error)
	UpdateStatus(ctx context.Context, scyllaDatacenter *v1alpha1.ScyllaDatacenter, opts v1.UpdateOptions) (*v1alpha1.ScyllaDatacenter, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ScyllaDatacenter, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ScyllaDatacenterList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ScyllaDatacenter, err error)
	ScyllaDatacenterExpansion
}

// scyllaDatacenters implements ScyllaDatacenterInterface
type scyllaDatacenters struct {
	client rest.Interface
	ns     string
}

// newScyllaDatacenters returns a ScyllaDatacenters
func newScyllaDatacenters(c *ScyllaV1alpha1Client, namespace string) *scyllaDatacenters {
	return &scyllaDatacenters{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the scyllaDatacenter, and returns the corresponding scyllaDatacenter object, and an error if there is any.
func (c *scyllaDatacenters) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ScyllaDatacenter, err error) {
	result = &v1alpha1.ScyllaDatacenter{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("scylladatacenters").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ScyllaDatacenters that match those selectors.
func (c *scyllaDatacenters) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ScyllaDatacenterList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ScyllaDatacenterList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("scylladatacenters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested scyllaDatacenters.
func (c *scyllaDatacenters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("scylladatacenters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a scyllaDatacenter and creates it.  Returns the server's representation of the scyllaDatacenter, and an error, if there is any.
func (c *scyllaDatacenters) Create(ctx context.Context, scyllaDatacenter *v1alpha1.ScyllaDatacenter, opts v1.CreateOptions) (result *v1alpha1.ScyllaDatacenter, err error) {
	result = &v1alpha1.ScyllaDatacenter{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("scylladatacenters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(scyllaDatacenter).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a scyllaDatacenter and updates it. Returns the server's representation of the scyllaDatacenter, and an error, if there is any.
func (c *scyllaDatacenters) Update(ctx context.Context, scyllaDatacenter *v1alpha1.ScyllaDatacenter, opts v1.UpdateOptions) (result *v1alpha1.ScyllaDatacenter, err error) {
	result = &v1alpha1.ScyllaDatacenter{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("scylladatacenters").
		Name(scyllaDatacenter.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(scyllaDatacenter).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *scyllaDatacenters) UpdateStatus(ctx context.Context, scyllaDatacenter *v1alpha1.ScyllaDatacenter, opts v1.UpdateOptions) (result *v1alpha1.ScyllaDatacenter, err error) {
	result = &v1alpha1.ScyllaDatacenter{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("scylladatacenters").
		Name(scyllaDatacenter.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(scyllaDatacenter).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the scyllaDatacenter and deletes it. Returns an error if one occurs.
func (c *scyllaDatacenters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("scylladatacenters").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *scyllaDatacenters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("scylladatacenters").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched scyllaDatacenter.
func (c *scyllaDatacenters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ScyllaDatacenter, err error) {
	result = &v1alpha1.ScyllaDatacenter{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("scylladatacenters").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
