// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scheme "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ScyllaManagersGetter has a method to return a ScyllaManagerInterface.
// A group's client should implement this interface.
type ScyllaManagersGetter interface {
	ScyllaManagers(namespace string) ScyllaManagerInterface
}

// ScyllaManagerInterface has methods to work with ScyllaManager resources.
type ScyllaManagerInterface interface {
	Create(ctx context.Context, scyllaManager *v1alpha1.ScyllaManager, opts v1.CreateOptions) (*v1alpha1.ScyllaManager, error)
	Update(ctx context.Context, scyllaManager *v1alpha1.ScyllaManager, opts v1.UpdateOptions) (*v1alpha1.ScyllaManager, error)
	UpdateStatus(ctx context.Context, scyllaManager *v1alpha1.ScyllaManager, opts v1.UpdateOptions) (*v1alpha1.ScyllaManager, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ScyllaManager, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ScyllaManagerList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ScyllaManager, err error)
	ScyllaManagerExpansion
}

// scyllaManagers implements ScyllaManagerInterface
type scyllaManagers struct {
	client rest.Interface
	ns     string
}

// newScyllaManagers returns a ScyllaManagers
func newScyllaManagers(c *ScyllaV1alpha1Client, namespace string) *scyllaManagers {
	return &scyllaManagers{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the scyllaManager, and returns the corresponding scyllaManager object, and an error if there is any.
func (c *scyllaManagers) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ScyllaManager, err error) {
	result = &v1alpha1.ScyllaManager{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("scyllamanagers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ScyllaManagers that match those selectors.
func (c *scyllaManagers) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ScyllaManagerList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ScyllaManagerList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("scyllamanagers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested scyllaManagers.
func (c *scyllaManagers) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("scyllamanagers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a scyllaManager and creates it.  Returns the server's representation of the scyllaManager, and an error, if there is any.
func (c *scyllaManagers) Create(ctx context.Context, scyllaManager *v1alpha1.ScyllaManager, opts v1.CreateOptions) (result *v1alpha1.ScyllaManager, err error) {
	result = &v1alpha1.ScyllaManager{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("scyllamanagers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(scyllaManager).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a scyllaManager and updates it. Returns the server's representation of the scyllaManager, and an error, if there is any.
func (c *scyllaManagers) Update(ctx context.Context, scyllaManager *v1alpha1.ScyllaManager, opts v1.UpdateOptions) (result *v1alpha1.ScyllaManager, err error) {
	result = &v1alpha1.ScyllaManager{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("scyllamanagers").
		Name(scyllaManager.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(scyllaManager).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *scyllaManagers) UpdateStatus(ctx context.Context, scyllaManager *v1alpha1.ScyllaManager, opts v1.UpdateOptions) (result *v1alpha1.ScyllaManager, err error) {
	result = &v1alpha1.ScyllaManager{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("scyllamanagers").
		Name(scyllaManager.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(scyllaManager).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the scyllaManager and deletes it. Returns an error if one occurs.
func (c *scyllaManagers) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("scyllamanagers").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *scyllaManagers) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("scyllamanagers").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched scyllaManager.
func (c *scyllaManagers) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ScyllaManager, err error) {
	result = &v1alpha1.ScyllaManager{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("scyllamanagers").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
