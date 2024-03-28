package collect

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/gather/collect/testhelpers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	kubefakeclient "k8s.io/client-go/kubernetes/fake"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/restmapper"
)

func TestCollector_CollectObject(t *testing.T) {
	t.Parallel()

	apiResources := []*metav1.APIResourceList{
		{
			GroupVersion: corev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "namespaces", Namespaced: false, Kind: "Namespace", Verbs: []string{"list"}},
				{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: []string{"list"}},
				{Name: "secrets", Namespaced: true, Kind: "Secret", Verbs: []string{"list"}},
			},
		},
		{
			GroupVersion: scyllav1.GroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "scyllaclusters", Namespaced: true, Kind: "ScyllaCluster", Verbs: []string{"list"}},
			},
		},
	}

	tt := []struct {
		name             string
		targetedObject   runtime.Object
		existingObjects  []runtime.Object
		relatedResources bool
		keepGoing        bool
		expectedDump     *testhelpers.GatherDump
		expectedError    error
	}{
		{
			name: "pod logs are skipped if there is no status",
			targetedObject: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "my-pod",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "my-container",
						},
					},
				},
			},
			existingObjects:  nil,
			relatedResources: false,
			keepGoing:        false,
			expectedError:    nil,
			expectedDump: &testhelpers.GatherDump{
				EmptyDirs: []string{
					"namespaces/test/pods/my-pod",
				},
				Files: []testhelpers.File{
					{
						Name: "namespaces/test/pods/my-pod.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: null
  name: my-pod
  namespace: test
spec:
  containers:
  - name: my-container
    resources: {}
status: {}
`, "\n"),
					},
				},
			},
		},
		{
			name: "fetches no pod logs from a container that didn't run yet",
			targetedObject: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "my-pod",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "my-container",
						},
					},
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "my-container",
							State: corev1.ContainerState{
								Terminated: nil,
								Running:    nil,
							},
						},
					},
				},
			},
			existingObjects:  nil,
			relatedResources: false,
			keepGoing:        false,
			expectedError:    nil,
			expectedDump: &testhelpers.GatherDump{
				EmptyDirs: []string{
					"namespaces/test/pods/my-pod",
				},
				Files: []testhelpers.File{
					{
						Name: "namespaces/test/pods/my-pod.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: null
  name: my-pod
  namespace: test
spec:
  containers:
  - name: my-container
    resources: {}
status:
  containerStatuses:
  - image: ""
    imageID: ""
    lastState: {}
    name: my-container
    ready: false
    restartCount: 0
    state: {}
`, "\n"),
					},
				},
			},
		},
		{
			name: "fetches only current pod logs from a new container that wasn't restarted",
			targetedObject: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "my-pod",
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "my-init-container",
						},
					},
					Containers: []corev1.Container{
						{
							Name: "my-container",
						},
					},
					EphemeralContainers: []corev1.EphemeralContainer{
						{
							EphemeralContainerCommon: corev1.EphemeralContainerCommon{
								Name: "my-ephemeral-container",
							},
						},
					},
				},
				Status: corev1.PodStatus{
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "my-init-container",
							State: corev1.ContainerState{
								Terminated: nil,
								Running:    &corev1.ContainerStateRunning{},
							},
						},
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "my-container",
							State: corev1.ContainerState{
								Terminated: nil,
								Running:    &corev1.ContainerStateRunning{},
							},
						},
					},
					EphemeralContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "my-ephemeral-container",
							State: corev1.ContainerState{
								Terminated: nil,
								Running:    &corev1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			existingObjects:  nil,
			relatedResources: false,
			keepGoing:        false,
			expectedError:    nil,
			expectedDump: &testhelpers.GatherDump{
				EmptyDirs: nil,
				Files: []testhelpers.File{
					{
						Name: "namespaces/test/pods/my-pod.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: null
  name: my-pod
  namespace: test
spec:
  containers:
  - name: my-container
    resources: {}
  ephemeralContainers:
  - name: my-ephemeral-container
    resources: {}
  initContainers:
  - name: my-init-container
    resources: {}
status:
  containerStatuses:
  - image: ""
    imageID: ""
    lastState: {}
    name: my-container
    ready: false
    restartCount: 0
    state:
      running:
        startedAt: null
  ephemeralContainerStatuses:
  - image: ""
    imageID: ""
    lastState: {}
    name: my-ephemeral-container
    ready: false
    restartCount: 0
    state:
      running:
        startedAt: null
  initContainerStatuses:
  - image: ""
    imageID: ""
    lastState: {}
    name: my-init-container
    ready: false
    restartCount: 0
    state:
      running:
        startedAt: null
`, "\n"),
					},
					{
						Name:    "namespaces/test/pods/my-pod/my-container.current",
						Content: "fake logs",
					},
					{
						Name:    "namespaces/test/pods/my-pod/my-ephemeral-container.current",
						Content: "fake logs",
					},
					{
						Name:    "namespaces/test/pods/my-pod/my-init-container.current",
						Content: "fake logs",
					},
				},
			},
		},
		{
			name: "fetches both current and previous pod logs from a container that was restarted",
			targetedObject: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "my-pod",
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "my-init-container",
						},
					},
					Containers: []corev1.Container{
						{
							Name: "my-container",
						},
					},
					EphemeralContainers: []corev1.EphemeralContainer{
						{
							EphemeralContainerCommon: corev1.EphemeralContainerCommon{
								Name: "my-ephemeral-container",
							},
						},
					},
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "my-container",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{},
							},
						},
					},
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "my-init-container",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{},
							},
						},
					},
					EphemeralContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "my-ephemeral-container",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{},
							},
						},
					},
				},
			},
			existingObjects:  nil,
			relatedResources: false,
			keepGoing:        false,
			expectedError:    nil,
			expectedDump: &testhelpers.GatherDump{
				EmptyDirs: nil,
				Files: []testhelpers.File{
					{
						Name: "namespaces/test/pods/my-pod.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: null
  name: my-pod
  namespace: test
spec:
  containers:
  - name: my-container
    resources: {}
  ephemeralContainers:
  - name: my-ephemeral-container
    resources: {}
  initContainers:
  - name: my-init-container
    resources: {}
status:
  containerStatuses:
  - image: ""
    imageID: ""
    lastState:
      terminated:
        exitCode: 0
        finishedAt: null
        startedAt: null
    name: my-container
    ready: false
    restartCount: 0
    state:
      running:
        startedAt: null
  ephemeralContainerStatuses:
  - image: ""
    imageID: ""
    lastState:
      terminated:
        exitCode: 0
        finishedAt: null
        startedAt: null
    name: my-ephemeral-container
    ready: false
    restartCount: 0
    state:
      running:
        startedAt: null
  initContainerStatuses:
  - image: ""
    imageID: ""
    lastState:
      terminated:
        exitCode: 0
        finishedAt: null
        startedAt: null
    name: my-init-container
    ready: false
    restartCount: 0
    state:
      running:
        startedAt: null
`, "\n"),
					},
					{
						Name:    "namespaces/test/pods/my-pod/my-container.current",
						Content: "fake logs",
					},
					{
						Name:    "namespaces/test/pods/my-pod/my-container.previous",
						Content: "fake logs",
					},
					{
						Name:    "namespaces/test/pods/my-pod/my-ephemeral-container.current",
						Content: "fake logs",
					},
					{
						Name:    "namespaces/test/pods/my-pod/my-ephemeral-container.previous",
						Content: "fake logs",
					},
					{
						Name:    "namespaces/test/pods/my-pod/my-init-container.current",
						Content: "fake logs",
					},
					{
						Name:    "namespaces/test/pods/my-pod/my-init-container.previous",
						Content: "fake logs",
					},
				},
			},
		},
		{
			name: "namespace doesn't collect any extra resources if related resources are disabled",
			targetedObject: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "my-namespace",
				},
			},
			existingObjects:  nil,
			relatedResources: false,
			keepGoing:        false,
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
  creationTimestamp: null
  name: my-namespace
  namespace: test
spec: {}
status: {}
`, "\n"),
					},
				},
			},
		},
		{
			name: "namespace collects all resources within if related resources are enabled",
			targetedObject: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-namespace",
				},
			},
			existingObjects: []runtime.Object{
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
			keepGoing:        false,
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
  creationTimestamp: null
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
  creationTimestamp: null
  name: my-secret
  namespace: my-namespace
`, "\n"),
					},
				},
			},
		},
		{
			name: "scyllacluster doesn't collect any extra resources if related resources are disabled",
			targetedObject: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "my-scyllacluster",
				},
			},
			existingObjects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "my-service",
					},
				},
			},
			relatedResources: false,
			keepGoing:        false,
			expectedError:    nil,
			expectedDump: &testhelpers.GatherDump{
				EmptyDirs: nil,
				Files: []testhelpers.File{
					{
						Name: "namespaces/test/scyllaclusters.scylla.scylladb.com/my-scyllacluster.yaml",
						Content: strings.TrimPrefix(`
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  creationTimestamp: null
  name: my-scyllacluster
  namespace: test
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
				},
			},
		},
		{
			name: "scyllacluster collects all resources in its namespace if related resources are enabled",
			targetedObject: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "my-scyllacluster",
				},
			},
			existingObjects: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "my-secret",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "other-namespace",
						Name:      "other-secret",
					},
				},
			},
			relatedResources: true,
			keepGoing:        false,
			expectedError:    nil,
			expectedDump: &testhelpers.GatherDump{
				EmptyDirs: nil,
				Files: []testhelpers.File{
					{
						Name: "cluster-scoped/namespaces/test.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Namespace
metadata:
  creationTimestamp: null
  name: test
spec: {}
status: {}
`, "\n"),
					},
					{
						Name: "namespaces/test/scyllaclusters.scylla.scylladb.com/my-scyllacluster.yaml",
						Content: strings.TrimPrefix(`
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  creationTimestamp: null
  name: my-scyllacluster
  namespace: test
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
						Name: "namespaces/test/secrets/my-secret.yaml",
						Content: strings.TrimPrefix(`
apiVersion: v1
kind: Secret
metadata:
  creationTimestamp: null
  name: my-secret
  namespace: test
`, "\n"),
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, ctxCancel := context.WithCancel(context.Background())
			defer ctxCancel()

			tmpDir := t.TempDir()

			scheme := runtime.NewScheme()
			err := corev1.AddToScheme(scheme)
			if err != nil {
				t.Fatal(err)
			}

			err = scyllav1.Install(scheme)
			if err != nil {
				t.Fatal(err)
			}

			u := &unstructured.Unstructured{}
			err = scheme.Convert(tc.targetedObject, u, nil)
			if err != nil {
				t.Fatal(err)
			}

			allObjects := make([]runtime.Object, 0, len(tc.existingObjects)+1)
			allObjects = append(allObjects, tc.targetedObject)
			allObjects = append(allObjects, tc.existingObjects...)

			var kubeObjects []runtime.Object
			var unstructuredObjects []runtime.Object
			for _, obj := range allObjects {
				_, _, err = kubernetesscheme.Scheme.ObjectKinds(obj)
				if err == nil {
					kubeObjects = append(kubeObjects, obj)
					unstructuredObjects = append(unstructuredObjects, obj)
				} else if runtime.IsNotRegisteredError(err) {
					unstructuredObjects = append(unstructuredObjects, obj)
				} else {
					t.Fatal(err)
				}
			}

			fakeKubeClient := kubefakeclient.NewSimpleClientset(kubeObjects...)
			fakeKubeClient.Resources = apiResources
			simpleFakeDiscoveryClient := fakeKubeClient.Discovery()
			fakeDiscoveryClient := &testhelpers.FakeDiscoveryWithSPR{
				FakeDiscovery: simpleFakeDiscoveryClient.(*fakediscovery.FakeDiscovery),
			}
			existingUnstructuredObjects := make([]runtime.Object, 0, len(unstructuredObjects))
			for _, e := range unstructuredObjects {
				u := &unstructured.Unstructured{}
				err := scheme.Convert(e, u, nil)
				if err != nil {
					t.Fatal(err)
				}
				existingUnstructuredObjects = append(existingUnstructuredObjects, u)
			}
			fakeDynamicClient := dynamicfakeclient.NewSimpleDynamicClient(scheme, existingUnstructuredObjects...)
			collector := NewCollector(
				tmpDir,
				[]ResourcePrinterInterface{
					&OmitManagedFieldsPrinter{Delegate: &YAMLPrinter{}},
				},
				fakeDiscoveryClient,
				fakeKubeClient.CoreV1(),
				fakeDynamicClient,
				tc.relatedResources,
				tc.keepGoing,
				0,
			)

			groupVersionKinds, _, err := scheme.ObjectKinds(tc.targetedObject)
			if err != nil {
				t.Fatal(err)
			}
			if len(groupVersionKinds) == 0 {
				t.Errorf("unsupported object type %T", tc.targetedObject)
			}
			if len(groupVersionKinds) > 1 {
				t.Errorf("mutiple kinds are not supported: %#v", groupVersionKinds)
			}
			gvk := groupVersionKinds[0]

			groupResources, err := restmapper.GetAPIGroupResources(fakeDiscoveryClient)
			if err != nil {
				t.Fatal(err)
			}
			discoveryMapper := restmapper.NewDiscoveryRESTMapper(groupResources)

			mapping, err := discoveryMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				t.Fatal(err)
			}

			err = collector.CollectObject(ctx, u, NewResourceInfoFromMapping(mapping))
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatal(err)
			}

			got, err := testhelpers.ReadGatherDump(tmpDir)
			if err != nil {
				t.Fatal(err)
			}

			diff := cmp.Diff(tc.expectedDump, got)
			if len(diff) != 0 {
				t.Errorf("expected and got filesystems differ:\n%s", diff)
			}
		})
	}
}
