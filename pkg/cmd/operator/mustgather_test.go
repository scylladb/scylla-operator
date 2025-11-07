package operator

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/gather/collect/testhelpers"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/spf13/cobra"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiserverinternalv1alpha1 "k8s.io/api/apiserverinternal/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kgenericclioptions "k8s.io/cli-runtime/pkg/genericclioptions"
	fakediscovery "k8s.io/client-go/discovery/fake"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	kubefakeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"
)

func TestMustGatherOptions_Run(t *testing.T) {
	t.Parallel()

	apiResources := []*metav1.APIResourceList{
		{
			GroupVersion: corev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "namespaces", Namespaced: false, Kind: "Namespace", Verbs: []string{"list"}},
				{Name: "nodes", Namespaced: false, Kind: "Node", Verbs: []string{"list"}},
				{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: []string{"list"}},
				{Name: "secrets", Namespaced: true, Kind: "Secret", Verbs: []string{"list"}},
			},
		},
		{
			GroupVersion: apiextensions.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "customresourcedefinitions", Namespaced: false, Kind: "CustomResourceDefinition", Verbs: []string{"list"}},
			},
		},
		{
			GroupVersion: admissionregistrationv1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "validatingwebhookconfigurations", Namespaced: false, Kind: "ValidatingWebhookConfiguration", Verbs: []string{"list"}},
				{Name: "mutatingwebhookconfigurations", Namespaced: false, Kind: "MutatingWebhookConfiguration", Verbs: []string{"list"}},
			},
		},
		{
			GroupVersion: apiserverinternalv1alpha1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "storageversions", Namespaced: true, Kind: "StorageVersion", Verbs: []string{"list"}},
			},
		},
		{
			GroupVersion: storagev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "storageclasses", Namespaced: false, Kind: "StorageClass", Verbs: []string{"list"}},
			},
		},
		{
			GroupVersion: scyllav1alpha1.GroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "scyllaoperatorconfigs", Namespaced: true, Kind: "ScyllaOperatorConfigs", Verbs: []string{"list"}},
				{Name: "nodeconfigs", Namespaced: true, Kind: "NodeConfigs", Verbs: []string{"list"}},
			},
		},
		{
			GroupVersion: scyllav1.GroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "scyllaclusters", Namespaced: true, Kind: "ScyllaCluster", Verbs: []string{"list"}},
			},
		},
	}

	testScheme := runtime.NewScheme()

	err := kubefakeclient.AddToScheme(testScheme)
	if err != nil {
		t.Fatal(err)
	}

	err = apiextensions.AddToScheme(testScheme)
	if err != nil {
		t.Fatal(err)
	}

	err = scyllav1alpha1.Install(testScheme)
	if err != nil {
		t.Fatal(err)
	}

	err = scyllav1.Install(testScheme)
	if err != nil {
		t.Fatal(err)
	}

	tt := []struct {
		name            string
		existingObjects []runtime.Object
		allResources    bool
		namespace       string
		expectedDump    *testhelpers.GatherDump
		expectedError   error
	}{
		{
			name:      "gathers objects in all namespaces",
			namespace: corev1.NamespaceAll,
			existingObjects: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-operator",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fedora",
					},
				},
				&storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-storage-class",
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-namespace",
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-other-namespace",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "scylla-operator",
						Name:      "my-secret",
					},
					Data: map[string][]byte{
						"secret-key": []byte("secret-value"),
					},
				},
				&scyllav1.ScyllaCluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "my-namespace",
						Name:      "scylla",
					},
				},
				&scyllav1.ScyllaCluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "my-other-namespace",
						Name:      "other-scylla",
					},
				},
				&apiserverinternalv1alpha1.StorageVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-non-standard-resource-that-should-not-be-collected",
					},
				},
			},
			allResources:  false,
			expectedError: nil,
			expectedDump: &testhelpers.GatherDump{
				EmptyDirs: nil,
				Files: []testhelpers.File{
					{
						Name: "cluster-scoped/namespaces/my-namespace.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Namespace
metadata:
  name: my-namespace
spec: {}
status: {}
`, "\n"),
					},
					{
						Name: "cluster-scoped/namespaces/my-other-namespace.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Namespace
metadata:
  name: my-other-namespace
spec: {}
status: {}
`, "\n"),
					},
					{
						Name: "cluster-scoped/namespaces/scylla-operator.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Namespace
metadata:
  name: scylla-operator
spec: {}
status: {}
`, "\n"),
					},
					{
						Name: "cluster-scoped/nodes/fedora.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Node
metadata:
  name: fedora
spec: {}
status:
  daemonEndpoints:
    kubeletEndpoint:
      Port: 0
  nodeInfo:
    architecture: ""
    bootID: ""
    containerRuntimeVersion: ""
    kernelVersion: ""
    kubeProxyVersion: ""
    kubeletVersion: ""
    machineID: ""
    operatingSystem: ""
    osImage: ""
    systemUUID: ""
`, "\n"),
					},
					{
						Name: "cluster-scoped/storageclasses.storage.k8s.io/my-storage-class.yaml",
						Content: strings.TrimPrefix(`
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: my-storage-class
provisioner: ""
`, "\n"),
					},
					{
						Name: "namespaces/my-namespace/scyllaclusters.scylla.scylladb.com/scylla.yaml",
						Content: strings.TrimPrefix(`
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: my-namespace
spec:
  agentVersion: ""
  datacenter:
    name: ""
    racks: null
  network: {}
  version: ""
status: {}
`, "\n"),
					},
					{
						Name: "namespaces/my-other-namespace/scyllaclusters.scylla.scylladb.com/other-scylla.yaml",
						Content: strings.TrimPrefix(`
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: other-scylla
  namespace: my-other-namespace
spec:
  agentVersion: ""
  datacenter:
    name: ""
    racks: null
  network: {}
  version: ""
status: {}
`, "\n"),
					},
					{
						Name: "scylla-operator-must-gather.log",
					},
				},
			},
		},
		{
			name:      "gathers objects only in selected namespace",
			namespace: "my-namespace",
			existingObjects: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-operator",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fedora",
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-namespace",
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-other-namespace",
					},
				},
				&scyllav1.ScyllaCluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "my-namespace",
						Name:      "scylla",
					},
				},
				&scyllav1.ScyllaCluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "my-other-namespace",
						Name:      "do-not-collect",
					},
				},
				&apiserverinternalv1alpha1.StorageVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-non-standard-resource-that-should-not-be-collected",
					},
				},
			},
			allResources:  false,
			expectedError: nil,
			expectedDump: &testhelpers.GatherDump{
				EmptyDirs: nil,
				Files: []testhelpers.File{
					{
						Name: "cluster-scoped/namespaces/my-namespace.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Namespace
metadata:
  name: my-namespace
spec: {}
status: {}
`, "\n"),
					},
					{
						Name: "cluster-scoped/namespaces/scylla-operator.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Namespace
metadata:
  name: scylla-operator
spec: {}
status: {}
`, "\n"),
					},
					{
						Name: "cluster-scoped/nodes/fedora.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Node
metadata:
  name: fedora
spec: {}
status:
  daemonEndpoints:
    kubeletEndpoint:
      Port: 0
  nodeInfo:
    architecture: ""
    bootID: ""
    containerRuntimeVersion: ""
    kernelVersion: ""
    kubeProxyVersion: ""
    kubeletVersion: ""
    machineID: ""
    operatingSystem: ""
    osImage: ""
    systemUUID: ""
`, "\n"),
					},
					{
						Name: "namespaces/my-namespace/scyllaclusters.scylla.scylladb.com/scylla.yaml",
						Content: strings.TrimPrefix(`
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: my-namespace
spec:
  agentVersion: ""
  datacenter:
    name: ""
    racks: null
  network: {}
  version: ""
status: {}
`, "\n"),
					},
					{
						Name: "scylla-operator-must-gather.log",
					},
				},
			},
		},
		{
			name:      "gathers all resources",
			namespace: corev1.NamespaceAll,
			existingObjects: []runtime.Object{
				&apiserverinternalv1alpha1.StorageVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-non-standard-resource",
					},
				},
			},
			allResources:  true,
			expectedError: nil,
			expectedDump: &testhelpers.GatherDump{
				EmptyDirs: nil,
				Files: []testhelpers.File{
					{
						Name: "namespaces/storageversions.internal.apiserver.k8s.io/my-non-standard-resource.yaml",
						Content: strings.TrimPrefix(`
apiVersion: internal.apiserver.k8s.io/v1alpha1
kind: StorageVersion
metadata:
  name: my-non-standard-resource
spec: {}
status: {}
`, "\n"),
					},
					{
						Name: "scylla-operator-must-gather.log",
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// klog uses global vars that have effect only on the first use, like log_file that create a logging stream
			// and doesn't refresh, so we need to use individual processes for this test and any similar test
			// in the same package.
			if os.Getenv("SOT_EXEC") != "1" {
				cmd := exec.Command(os.Args[0], fmt.Sprintf("-test.run=%s", t.Name()))
				cmd.Env = append(os.Environ(), "SOT_EXEC=1")
				var stdout bytes.Buffer
				cmd.Stdout = &stdout
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				t.Logf("Running worker comamnd %q", cmd.String())
				err := cmd.Run()
				t.Logf("Finished running worker command %q.\nStdOut:\n%s\nStdErr:\n%s", cmd.String(), stdout.String(), stderr.String())
				exitErr := &exec.ExitError{}
				if err == nil || errors.As(err, &exitErr) && exitErr.Success() {
					return
				} else {
					t.Fatalf("command %q exited with error %v", cmd.String(), err)
					return
				}
			}

			tmpDir := t.TempDir()

			// We don't actually list objects using this client and it can only work with native APIs,
			// so we can leave it empty.
			fakeKubeClient := kubefakeclient.NewSimpleClientset()
			fakeKubeClient.Resources = apiResources
			simpleFakeDiscoveryClient := fakeKubeClient.Discovery()
			fakeDiscoveryClient := &testhelpers.FakeDiscoveryWithSPR{
				FakeDiscovery: simpleFakeDiscoveryClient.(*fakediscovery.FakeDiscovery),
			}
			existingUnstructuredObjects := make([]runtime.Object, 0, len(tc.existingObjects))
			for _, e := range tc.existingObjects {
				u := &unstructured.Unstructured{}
				err := testScheme.Convert(e, u, nil)
				if err != nil {
					t.Fatal(err)
				}
				existingUnstructuredObjects = append(existingUnstructuredObjects, u)
			}
			fakeDynamicClient := dynamicfakeclient.NewSimpleDynamicClient(testScheme, existingUnstructuredObjects...)
			streams := genericclioptions.IOStreams{
				In:     os.Stdin,
				Out:    os.Stdout,
				ErrOut: os.Stderr,
			}
			o := NewMustGatherOptions(streams)
			o.cliFlags.DestDir = tmpDir
			o.kubeClient = fakeKubeClient
			o.discoveryClient = fakeDiscoveryClient
			o.dynamicClient = fakeDynamicClient
			o.AllResources = tc.allResources
			o.GatherBaseOptions.ConfigFlags = kgenericclioptions.NewConfigFlags(false)
			*o.GatherBaseOptions.ConfigFlags.Namespace = tc.namespace

			cmd := &cobra.Command{
				Use: "must-gather",
			}
			o.AddFlags(cmd.Flags())
			klog.InitFlags(nil)
			err = o.Run(streams, cmd)
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %#v, got %#v", tc.expectedError, err)
			}

			got, err := testhelpers.ReadGatherDump(tmpDir)
			if err != nil {
				t.Fatal(err)
			}

			// The log has time stamps and other variable input, let's test only it's presence for now.
			// Eventually we can come back and see about reducing / replacing the variable input.
			for i := range got.Files {
				f := &got.Files[i]
				if f.Name == "scylla-operator-must-gather.log" {
					f.Content = ""
				}
			}

			diff := cmp.Diff(tc.expectedDump, got)
			if len(diff) != 0 {
				t.Errorf("expected and got dumps differ:\n%s", diff)
			}
		})
	}
}
