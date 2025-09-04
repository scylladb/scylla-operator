package operator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/gather/collect/testhelpers"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kgenericclioptions "k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	fakediscovery "k8s.io/client-go/discovery/fake"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	kubefakeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"
)

func TestGatherOptions_Run(t *testing.T) {
	t.Parallel()

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	apiResources := []*metav1.APIResourceList{
		{
			GroupVersion: corev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "namespaces", Namespaced: false, Kind: "Namespace", Verbs: []string{"list"}},
				{Name: "secrets", Namespaced: true, Kind: "Secret", Verbs: []string{"list"}},
			},
		},
	}

	testScheme := runtime.NewScheme()

	err := corev1.AddToScheme(testScheme)
	if err != nil {
		t.Fatal(err)
	}

	tt := []struct {
		name             string
		infos            []*resource.Info
		existingObjects  []runtime.Object
		relatedResources bool
		expectedDump     *testhelpers.GatherDump
		expectedError    error
	}{
		{
			name: "smoke test",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{
							Group:    corev1.SchemeGroupVersion.Group,
							Version:  corev1.SchemeGroupVersion.Version,
							Resource: "namespaces",
						},
						Scope: meta.RESTScopeRoot,
					},
					Name: "my-namespace",
				},
			},
			existingObjects: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-namespace",
						Annotations: map[string]string{
							"annotation-key": "annotation-value",
						},
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "my-namespace",
						Name:      "my-secret",
					},
					Data: map[string][]byte{
						"secret-key": []byte("secret-value"),
					},
				},
			},
			relatedResources: true,
			expectedError:    nil,
			expectedDump: &testhelpers.GatherDump{
				EmptyDirs: nil,
				Files: []testhelpers.File{
					{
						Name: "cluster-scoped/namespaces/my-namespace.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    annotation-key: annotation-value
  name: my-namespace
spec: {}
status: {}
`, "\n"),
					},
					{
						Name: "namespaces/my-namespace/secrets/my-secret.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
data:
  secret-key: PHJlZGFjdGVkPg==
kind: Secret
metadata:
  name: my-secret
  namespace: my-namespace
`, "\n"),
					},
					{
						Name: "scylla-operator-gather.log",
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
				if err == nil || (errors.As(err, &exitErr) && exitErr.Success()) {
					return
				} else {
					t.Fatalf("command %q exited with error %v", cmd.String(), err)
					return
				}
			}

			tmpDir := t.TempDir()

			fakeKubeClient := kubefakeclient.NewSimpleClientset(tc.existingObjects...)
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
			o := NewGatherOptions(streams)
			o.DestDir = tmpDir
			o.kubeClient = fakeKubeClient
			o.discoveryClient = fakeDiscoveryClient
			o.dynamicClient = fakeDynamicClient

			for _, info := range tc.infos {
				obj, err := fakeDynamicClient.Resource(info.Mapping.Resource).Namespace(info.Namespace).Get(ctx, info.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				info.Object = obj
			}
			o.builder = kgenericclioptions.NewSimpleFakeResourceFinder(tc.infos...)

			cmd := &cobra.Command{
				Use: "gather",
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
				if f.Name == "scylla-operator-gather.log" {
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
