//go:build envtest

package controllers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllermanager"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The manager's client is what the controllers, ObjectControl and kubecrypto among them, trust to observe their own
// writes from the cache. The Secret watch stream goes through a gate the spec holds around every write. While the
// gate is held nothing reaches the cache, so a read without the guarantee has to miss, and a read with it has to
// wait: a live read would return the object, a plain cached read the stale state, and neither would block.
var _ = g.Describe("Read-your-writes client", func() {
	// cacheWaitTimeout bounds a read that waits for the cache while the gate is held. It can't be too short: with the
	// gate held the cache can't catch up, whatever the duration.
	const cacheWaitTimeout = time.Second

	g.It("reads from the cache and waits for its own writes to reach it", func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)

		g.By("Starting the controller manager with the Secret watch stream running through a gate")
		gate := newWatchGate()
		g.DeferCleanup(gate.Release)
		cm := runControllerManager(ctx, env, func(options *controllermanager.Options) {
			options.RestConfig = rest.CopyConfig(options.RestConfig)
			options.RestConfig.WrapTransport = transport.Wrappers(options.RestConfig.WrapTransport, gate.holdWatches("secrets"))
		})

		g.By("Waiting for the controller manager to be running")
		waitForScyllaOperatorConfigSingleton(ctx, env)

		c := cm.Client().Client()
		g.By("Waiting for the Secret informer to be synced, then holding the gate")
		// The informer syncs over the watch stream, so the gate has to stay open until then. A cached read blocks
		// until the informer is synced.
		err := c.Get(ctx, client.ObjectKey{Namespace: env.Namespace(), Name: "never"}, &corev1.Secret{}, client.DisableReadYourWritesConsistency)
		o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "expected a not found error, got %v", err)
		gate.Hold()

		// getWithNoCacheConsistency reads the cache without the guarantee, to show the cache is behind.
		getWithNoCacheConsistency := func(key client.ObjectKey) (*corev1.Secret, error) {
			secret := &corev1.Secret{}
			err := c.Get(ctx, key, secret, client.DisableReadYourWritesConsistency)
			return secret, err
		}
		// expectReadToWaitForCache reads with the gate held and expects the read to wait for the cache until its
		// deadline instead of returning what the cache or the API server hold.
		expectReadToWaitForCache := func(key client.ObjectKey) {
			g.GinkgoHelper()

			readCtx, cancel := context.WithTimeout(ctx, cacheWaitTimeout)
			defer cancel()
			err := c.Get(readCtx, key, &corev1.Secret{})
			o.Expect(errors.Is(err, context.DeadlineExceeded)).To(o.BeTrue(), "expected the read to wait for the cache until its deadline, got %v", err)
		}
		// getThroughOpenGate opens the gate for one read and holds it again once the read returns.
		getThroughOpenGate := func(key client.ObjectKey) (*corev1.Secret, error) {
			gate.Release()
			defer gate.Hold()

			secret := &corev1.Secret{}
			err := c.Get(ctx, key, secret)
			return secret, err
		}

		g.By("Creating a Secret and reading it back")
		created := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: env.Namespace(),
				Name:      "read-your-writes",
			},
			StringData: map[string]string{"key": "v1"},
		}
		err = c.Create(ctx, created)
		o.Expect(err).NotTo(o.HaveOccurred())
		key := client.ObjectKeyFromObject(created)
		_, err = getWithNoCacheConsistency(key)
		o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "expected the cache to be behind the create, got %v", err)
		expectReadToWaitForCache(key)
		got, err := getThroughOpenGate(key)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(got.UID).To(o.Equal(created.UID))

		g.By("Updating it and reading the update back")
		updated := got.DeepCopy()
		updated.StringData = map[string]string{"key": "v2"}
		err = c.Update(ctx, updated)
		o.Expect(err).NotTo(o.HaveOccurred())
		cachedSecret, err := getWithNoCacheConsistency(key)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cachedSecret.ResourceVersion).To(o.Equal(created.ResourceVersion), "expected the cache to be behind the update")
		expectReadToWaitForCache(key)
		got, err = getThroughOpenGate(key)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(got.ResourceVersion).To(o.Equal(updated.ResourceVersion))

		g.By("Deleting it and reading the deletion back")
		err = c.Delete(ctx, updated)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = getWithNoCacheConsistency(key)
		o.Expect(err).NotTo(o.HaveOccurred(), "expected the cache to be behind the delete")
		expectReadToWaitForCache(key)
		_, err = getThroughOpenGate(key)
		o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "expected a not found error, got %v", err)
	})
})

// watchGate holds the chunks of a watch stream while held and lets them through while released. Every chunk is read
// off the wire as it arrives and handed on only when the gate is released. It starts released.
type watchGate struct {
	mu       sync.Mutex
	released chan struct{}
}

func newWatchGate() *watchGate {
	released := make(chan struct{})
	close(released)
	return &watchGate{
		released: released,
	}
}

// Release lets the held chunks and every following one through, until Hold.
func (wg *watchGate) Release() {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	select {
	case <-wg.released:
	default:
		close(wg.released)
	}
}

// Hold makes the gate hold the chunks that arrive from now on.
func (wg *watchGate) Hold() {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	select {
	case <-wg.released:
		wg.released = make(chan struct{})
	default:
	}
}

func (wg *watchGate) wait() {
	wg.mu.Lock()
	released := wg.released
	wg.mu.Unlock()

	<-released
}

// holdWatches returns a transport wrapper that runs the watch responses for resource through the gate. Other requests
// are untouched.
func (wg *watchGate) holdWatches(resource string) transport.WrapperFunc {
	return func(rt http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			resp, err := rt.RoundTrip(req)
			if err != nil || req.URL.Query().Get("watch") != "true" || path.Base(req.URL.Path) != resource {
				return resp, err
			}

			resp.Body = gatedBody{
				ReadCloser: resp.Body,
				gate:       wg,
			}
			return resp, nil
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type gatedBody struct {
	io.ReadCloser
	gate *watchGate
}

func (b gatedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.gate.wait()
	return n, err
}
