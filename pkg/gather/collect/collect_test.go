package collect

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/gather/collect/testhelpers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	kubefakeclient "k8s.io/client-go/kubernetes/fake"
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
	}

	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	if err != nil {
		t.Fatal(err)
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
						Name: IntegrityFileName,
						Content: strings.TrimPrefix(`
checksum: d763fa8cc6665af15380368055bb31a5720ef80d7e49232a4527e450d3d8a5fa129cac17068863ce63374afb32494089462ad79e47898b48a3f24957334fd4e8
directories:
  /namespaces/test/pods:
    checksum: d763fa8cc6665af15380368055bb31a5720ef80d7e49232a4527e450d3d8a5fa129cac17068863ce63374afb32494089462ad79e47898b48a3f24957334fd4e8
    fileCount: 1
`, "\n"),
					},
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
						Name: IntegrityFileName,
						Content: strings.TrimPrefix(`
checksum: d2503c86cb530b941dd9239ee48033d804b5e7d1e4861b0b116d205b38232c9564ed13634b96901f1e471777b419754962bbe559921a34b0bb0b54b8e11fe303
directories:
  /namespaces/test/pods:
    checksum: d2503c86cb530b941dd9239ee48033d804b5e7d1e4861b0b116d205b38232c9564ed13634b96901f1e471777b419754962bbe559921a34b0bb0b54b8e11fe303
    fileCount: 1
`, "\n"),
					},
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
						Name: IntegrityFileName,
						Content: strings.TrimPrefix(`
checksum: 25149bc2e1d1c698989b4f6c9d6a9d87bd51369bc9328b2923ab1cddda62ecc397f254634a184da550a3b39d114d7c19a69cfd7cb266576907656ba1e6535bd7
directories:
  /namespaces/test/pods:
    checksum: 25149bc2e1d1c698989b4f6c9d6a9d87bd51369bc9328b2923ab1cddda62ecc397f254634a184da550a3b39d114d7c19a69cfd7cb266576907656ba1e6535bd7
    fileCount: 1
`, "\n"),
					},
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
						Name: IntegrityFileName,
						Content: strings.TrimPrefix(`
checksum: 35795801b54e77cfbe1567452e2ab7053631a02696dcc7418e173c6db99fb4dc0fab98f444b6020be77da398e8b147a47b08c64a3bfb8e96912f4aa67c04e677
directories:
  /namespaces/test/pods:
    checksum: 35795801b54e77cfbe1567452e2ab7053631a02696dcc7418e173c6db99fb4dc0fab98f444b6020be77da398e8b147a47b08c64a3bfb8e96912f4aa67c04e677
    fileCount: 1
`, "\n"),
					},
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
						Name: IntegrityFileName,
						Content: strings.TrimPrefix(`
checksum: 3b11d89fb3b46dd29852f0efd5eefbced9d664f0d7b93aefa8532d62c856c6a6ba0cc7eb1f898d1c337d6827d77a4d325e6ae6c555f0bf0b0973819bf85a1c51
directories:
  /cluster-scoped/namespaces:
    checksum: 3b11d89fb3b46dd29852f0efd5eefbced9d664f0d7b93aefa8532d62c856c6a6ba0cc7eb1f898d1c337d6827d77a4d325e6ae6c555f0bf0b0973819bf85a1c51
    fileCount: 1
`, "\n"),
					},
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
						Name: IntegrityFileName,
						Content: strings.TrimPrefix(`
checksum: 7f340ac7a111169126248f69c955b347d6ff5422e0cc792031bd8398dc766137896fcc188aa3ccbc274a6128296572e971dde235fc050cbf1baaa1af3a6b397a
directories:
  /cluster-scoped/namespaces:
    checksum: 03cb220813bf01d43c5c8316e4a3b5a766770fa3437196ddaabc478269025570f33c93f6c54791af2b450e3b197af0df5ca8ec4c3285b427c743c40b771fc225
    fileCount: 1
  /namespaces/my-namespace/secrets:
    checksum: 99a521ab8ba30d21687b0eca6ffa1f4a4a98c2ca05a016d5f215b9a8690820abbbfddf63be1e486392a236c2c91b2a19e0ec50644c97ecb6d4b39406ecaa2e48
    fileCount: 1
`, "\n"),
					},
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
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, ctxCancel := context.WithCancel(context.Background())
			defer ctxCancel()

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

			u := &unstructured.Unstructured{}
			err = scheme.Convert(tc.targetedObject, u, nil)
			if err != nil {
				t.Fatal(err)
			}

			err = collector.CollectObject(ctx, u, NewResourceInfoFromMapping(mapping))
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatal(err)
			}

			_, err = collector.WriteIntegrityFile()
			if err != nil {
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
