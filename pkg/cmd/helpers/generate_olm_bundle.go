package helpers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
	ofversion "github.com/operator-framework/api/pkg/lib/version"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/yaml"
)

var (
	manifestsScheme = runtime.NewScheme()
	manifestsCodecs = serializer.NewCodecFactory(manifestsScheme)

	examplesScheme = runtime.NewScheme()
	examplesCodecs = serializer.NewCodecFactory(examplesScheme)
)

func init() {
	utilruntime.Must(operatorsv1alpha1.AddToScheme(manifestsScheme))
	kubernetesscheme.AddToScheme(manifestsScheme)

	utilruntime.Must(scyllav1.Install(examplesScheme))
}

type GenerateOLMBundleOptions struct {
	Version               string
	CSVPath               string
	IconPath              string
	ExamplePaths          []string
	ImageReplacementSpecs []string
	InputManifestPaths    []string
	OutputDir             string

	operatorVersion     ofversion.OperatorVersion
	IconMIME            string
	imageReplacementMap map[string]string
}

func NewGenerateOLMBundleOptions(streams genericclioptions.IOStreams) *GenerateOLMBundleOptions {
	return &GenerateOLMBundleOptions{
		imageReplacementMap: map[string]string{},
	}
}

func NewGenerateOLMBundleCommand(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewGenerateOLMBundleOptions(streams)

	cmd := &cobra.Command{
		Use:                   "generate-olm-bundle [flags] MANIFESTS...",
		DisableFlagsInUseLine: true,
		Short:                 "Generates OLM bundle from deployment manifests.",
		Long:                  `Generates OLM bundle from deployment manifests.`,
		Args:                  cobra.ArbitraryArgs,
		Example: templates.Examples(`
			# Generate OLM bundle over an existing one.
			helpers generate-olm-bundle --version=0.0.2-latest --csv-file=./deploy/olm/latest/manifests/scyllaoperator.clusterserviceversion.yaml --icon-file ./deploy/olm/latest/icon.svg --output-dir=./deploy/olm/latest/ $( find ./deploy/olm/latest/examples/ -name '*.yaml' -printf '--example-file=%p ' ) --loglevel=4 -- ./deploy/manifests/operator/*.yaml
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.ParseArgs(cmd.ArgsLenAtDash(), args)
			if err != nil {
				return err
			}

			err = o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd.Name())
			if err != nil {
				return err
			}

			return nil
		},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.Flags().StringVarP(&o.Version, "version", "", o.Version, "Version string that goes into csv.spec.version.")
	cmd.Flags().StringVarP(&o.CSVPath, "csv-file", "", o.CSVPath, "A manifest file containing ClusterServiceVersion object.")
	cmd.Flags().StringVarP(&o.IconPath, "icon-file", "", o.IconPath, "Icon file.")
	cmd.Flags().StringVarP(&o.IconMIME, "icon-mime", "", o.IconMIME, "Icon MIME type. Inferred from the icon file extension, if empty.")
	cmd.Flags().StringArrayVarP(&o.ExamplePaths, "example-file", "", o.ExamplePaths, "A file containing the example to embed.")
	cmd.Flags().StringArrayVarP(&o.ImageReplacementSpecs, "replace-image", "", o.ImageReplacementSpecs, "Replaces image reference in the manifests with a new one. Using `FROM=TO` replaces image reference `FROM` with `TO`. Exits non-zero if there wasn't any instance to be replaced. ")
	cmd.Flags().StringVarP(&o.OutputDir, "output-dir", "", o.OutputDir, "Output directory for the OLM bundle.")

	return cmd
}

func (o *GenerateOLMBundleOptions) ParseArgs(argsLenAtDash int, args []string) error {
	if argsLenAtDash > 0 {
		return fmt.Errorf("no argument is supported before '--', received %#v", args[:argsLenAtDash])
	}

	o.InputManifestPaths = args

	return nil
}

func (o *GenerateOLMBundleOptions) Validate() error {
	var errs []error

	if len(o.Version) == 0 {
		errs = append(errs, errors.New("version can't be empty"))
	}

	if len(o.CSVPath) == 0 {
		errs = append(errs, errors.New("csv-file can't be empty"))
	}

	if len(o.IconPath) == 0 {
		errs = append(errs, errors.New("icon-file can't be empty"))
	}

	if len(o.InputManifestPaths) == 0 {
		errs = append(errs, errors.New("input manifests can't be empty"))
	}

	if len(o.OutputDir) == 0 {
		errs = append(errs, errors.New("output-dir can't be empty"))
	}

	fileInfo, err := os.Stat(o.OutputDir)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't stat output-dir: %w", err))
	}
	if !fileInfo.IsDir() {
		errs = append(errs, errors.New("output-dir isn't a directory"))
	}

	return apierrors.NewAggregate(errs)
}

func (o *GenerateOLMBundleOptions) Complete() error {
	var err error

	o.operatorVersion.Version, err = semver.Parse(o.Version)
	if err != nil {
		return fmt.Errorf("can't parse version %q: %w", o.Version, err)
	}

	if len(o.IconMIME) == 0 {
		IconExt := filepath.Ext(o.IconPath)
		o.IconMIME = mime.TypeByExtension(IconExt)
		if len(o.IconMIME) == 0 {
			return fmt.Errorf("file %q: can't detect MIME type from extension %q", o.IconPath, IconExt)
		}
	}

	for _, irs := range o.ImageReplacementSpecs {
		parts := strings.SplitN(irs, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("can't parse --replace-image value %q: must be valid FROM=TO mapping", irs)
		}
		from := parts[0]
		to := parts[1]
		if len(from) == 0 {
			return fmt.Errorf(`can't parse --replace-image value %q: "FROM" is empty`, irs)
		}
		if len(to) == 0 {
			return fmt.Errorf(`can't parse --replace-image value %q: "TO" is empty`, irs)
		}

		_, found := o.imageReplacementMap[from]
		if found {
			return fmt.Errorf("duplicate image replacement %q: key %q already exists", irs, from)
		}

		o.imageReplacementMap[from] = to
	}

	return nil
}

func (o *GenerateOLMBundleOptions) Run(streams genericclioptions.IOStreams, commandName string) error {
	klog.Infof("%s version %s", commandName, version.Get())
	klog.Infof("loglevel is set to %q", cmdutil.GetLoglevel())

	stopCh := signals.StopChannel()
	_, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()
	go func() {
		<-stopCh
		ctxCancel()
	}()

	klog.V(4).Infof("input manifests: %v", o.InputManifestPaths)

	csvData, err := ioutil.ReadFile(o.CSVPath)
	if err != nil {
		return fmt.Errorf("can't read csv file %q: %w", o.CSVPath, err)
	}

	csvGVK := operatorsv1alpha1.SchemeGroupVersion.WithKind("ClusterServiceVersion")
	csvObj, _, err := manifestsCodecs.UniversalDeserializer().Decode(csvData, &csvGVK, nil)
	if err != nil {
		return fmt.Errorf("can't decode csv file %q: %w", o.CSVPath, err)
	}

	objGVK := csvObj.GetObjectKind().GroupVersionKind()
	if objGVK != csvGVK {
		return fmt.Errorf("csv file contains object with GVK %q, expected %q", objGVK, csvGVK)
	}

	csv, ok := csvObj.(*operatorsv1alpha1.ClusterServiceVersion)
	if !ok {
		return fmt.Errorf("couldn't cast csv object of type %T to *operatorsv1alpha1.ClusterServiceVersion", csvObj)
	}

	klog.V(2).Infof("Parsed CSV %q", csv.Name)

	unstructuredManifests := map[string]*unstructured.Unstructured{}
	for _, manifestPath := range o.InputManifestPaths {
		manifestData, err := ioutil.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("can't read file %q: %w", manifestPath, err)
		}

		for from, to := range o.imageReplacementMap {
			manifestData = []byte(strings.ReplaceAll(string(manifestData), from, to))
		}

		// YAML is a subset of JSON so this will be noop if it's JSON already.
		manifestJSON, err := yaml.YAMLToJSON(manifestData)
		if err != nil {
			return fmt.Errorf("can't convert manifest %q from YAML to JSON: %w", manifestPath, err)
		}

		manifestObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, manifestJSON)
		if err != nil {
			return fmt.Errorf("can't decode manifest %q: %w", manifestPath, err)
		}

		manifestUnstructured, ok := manifestObj.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("manifests %q: can't cast object of type %T to unstructured", manifestPath, manifestObj)
		}

		manifestGVK := manifestUnstructured.GroupVersionKind()

		switch manifestGVK.GroupVersion() {
		case schema.GroupVersion{
			Group:   "cert-manager.io",
			Version: "v1",
		}:
			// OLM handles webhook certs internally, ignore cert-manager manifests.
			continue
		}

		switch manifestGVK {
		case schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Namespace",
		}:
			// OLM handles namespaces internally.
			continue
		}

		unstructuredManifests[manifestPath] = manifestUnstructured
	}

	// We need to parse the examples to make sure the structure matches the scheme. Decoding and encoding
	// again will make sure that we prune any fields that aren't matching the scheme.
	var examples []metav1.Object
	for _, examplePath := range o.ExamplePaths {
		data, err := ioutil.ReadFile(examplePath)
		if err != nil {
			return fmt.Errorf("can't read example file %q: %w", examplePath, err)
		}

		obj, _, err := examplesCodecs.UniversalDeserializer().Decode(data, nil, nil)
		if err != nil {
			return fmt.Errorf("can't decode example file %q: %w", examplePath, err)
		}

		metaObj, err := meta.Accessor(obj)
		if err != nil {
			return fmt.Errorf("can't access object meta in example file %q: %w", examplePath, err)
		}

		examples = append(examples, metaObj)
	}

	type serviceWithPath struct {
		path    string
		service *corev1.Service
	}
	servicesWithPath := map[string]serviceWithPath{}
	serviceGVK := corev1.SchemeGroupVersion.WithKind("Service")
	for p, u := range unstructuredManifests {
		if u.GroupVersionKind() != serviceGVK {
			continue
		}

		svc := &corev1.Service{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), svc)
		if err != nil {
			return fmt.Errorf("can't convert %T to %T: %w", u, svc, err)
		}

		svcKey := naming.ObjRef(svc)
		existingService, found := servicesWithPath[svcKey]
		if found {
			return fmt.Errorf("can't add service %q (file %q): service already exists (file %q)", svcKey, p, existingService.path)
		}

		servicesWithPath[svcKey] = serviceWithPath{
			path:    p,
			service: svc,
		}
	}
	klog.V(4).Infof("Found %d service(s)", len(servicesWithPath))

	iconData, err := ioutil.ReadFile(o.IconPath)
	if err != nil {
		return fmt.Errorf("can't read icon file %q: %w", o.IconPath, err)
	}

	// At this point we have processed all the inputs and have all the input data parsed and saved in memory.
	// This is important to prevent parsing errors later on and also to be able to write into the same files.

	// Embed resources into the CSV. Sort arrays for consistent output.

	// Embed version.
	csv.Name = fmt.Sprintf("scyllaoperator.v%s", o.operatorVersion)
	csv.Spec.Version = o.operatorVersion

	// Embed icon.
	csv.Spec.Icon = []operatorsv1alpha1.Icon{
		{
			Data:      base64.StdEncoding.EncodeToString(iconData),
			MediaType: o.IconMIME,
		},
	}

	// Embed examples.

	klog.V(2).Infof("Embedding %d example(s)", len(examples))

	sort.Slice(examples, func(i, j int) bool {
		return objLess(examples[i], examples[j])
	})

	examplesBytes, err := yaml.Marshal(examples)
	if err != nil {
		return fmt.Errorf("can't marshal examples: %w", err)
	}
	if csv.Annotations == nil {
		csv.Annotations = map[string]string{}
	}
	csv.Annotations["alm-examples"] = string(examplesBytes)

	// Embed CRDs.

	crdGVK := apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition")
	csv.Spec.CustomResourceDefinitions.Owned = nil
	for k, u := range unstructuredManifests {
		if u.GroupVersionKind() != crdGVK {
			continue
		}

		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), crd)
		if err != nil {
			return fmt.Errorf("can't convert %T to %T: %w", u, crd, err)
		}

		for _, crdVersion := range crd.Spec.Versions {
			csv.Spec.CustomResourceDefinitions.Owned = append(csv.Spec.CustomResourceDefinitions.Owned, operatorsv1alpha1.CRDDescription{
				Name:        crd.Name,
				Kind:        crd.Spec.Names.Kind,
				Version:     crdVersion.Name,
				DisplayName: crd.Spec.Names.Kind,
				Description: crdVersion.Schema.OpenAPIV3Schema.Description,
			})
		}
		delete(unstructuredManifests, k)
	}

	klog.V(2).Infof("Embedding %d crds(s)", len(csv.Spec.CustomResourceDefinitions.Owned))

	// Embed Deployments.

	deploymentGVK := appsv1.SchemeGroupVersion.WithKind("Deployment")
	var deployments []*appsv1.Deployment
	for k, u := range unstructuredManifests {
		if u.GroupVersionKind() != deploymentGVK {
			continue
		}

		deployment := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), deployment)
		if err != nil {
			return fmt.Errorf("can't convert %T to %T: %w", u, deployment, err)
		}

		deployments = append(deployments, deployment)
		delete(unstructuredManifests, k)
	}

	if len(deployments) == 0 {
		return fmt.Errorf("there must be at least one deployment to embed")
	}

	klog.V(2).Infof("Embedding %d deployment(s)", len(deployments))

	sort.Slice(deployments, func(i, j int) bool {
		return objLess(deployments[i], deployments[j])
	})

	// FIXME: embed image refs
	// TODO: consider resolving into sha

	csv.Spec.InstallStrategy.StrategyName = "deployment"
	csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs = nil
	for _, d := range deployments {
		d := d.DeepCopy()
		csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs = append(csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs, operatorsv1alpha1.StrategyDeploymentSpec{
			Name:  d.Name,
			Spec:  d.Spec,
			Label: d.Labels,
		})
	}

	// Embed webhooks.

	csv.Spec.WebhookDefinitions = nil

	validatingWebhookConfigurationGVK := admissionregistrationv1.SchemeGroupVersion.WithKind("ValidatingWebhookConfiguration")
	var validatingWebhookDefinitions []operatorsv1alpha1.WebhookDescription
	for p, u := range unstructuredManifests {
		if u.GroupVersionKind() != validatingWebhookConfigurationGVK {
			continue
		}

		validatingWebhookConfiguration := &admissionregistrationv1.ValidatingWebhookConfiguration{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), validatingWebhookConfiguration)
		if err != nil {
			return fmt.Errorf("can't convert %T to %T: %w", u, validatingWebhookConfiguration, err)
		}

		for i, w := range validatingWebhookConfiguration.Webhooks {
			if w.ClientConfig.Service == nil {
				return fmt.Errorf("file %q: olm supports only service webhhoks", p)
			}

			svcKey := naming.ObjRef(&metav1.ObjectMeta{
				Namespace: w.ClientConfig.Service.Namespace,
				Name:      w.ClientConfig.Service.Name,
			})
			svcWithPath, found := servicesWithPath[svcKey]
			if !found {
				return fmt.Errorf(
					"file %q: ValidatingWebhookConfiguration %q webhook %d references unknown service %q",
					p,
					naming.ObjRef(validatingWebhookConfiguration),
					i,
					svcKey,
				)
			}

			delete(unstructuredManifests, svcWithPath.path)

			d, err := findDeploymentForService(svcWithPath.service, deployments)
			if err != nil {
				return fmt.Errorf("file %q: can't find deployment from service: %w", p, err)
			}

			webhookPort := int32(443)
			if w.ClientConfig.Service.Port != nil {
				webhookPort = *w.ClientConfig.Service.Port
			}

			port, err := findServicePort(svcWithPath.service, webhookPort)
			if err != nil {
				return fmt.Errorf("file %q: can't find deployment for service: %w", p, err)
			}

			validatingWebhookDefinitions = append(validatingWebhookDefinitions, operatorsv1alpha1.WebhookDescription{
				DeploymentName:          d.Name,
				AdmissionReviewVersions: w.AdmissionReviewVersions,
				// ContainerPort in OLM actually means the service port,
				// not the target port which is equivalent to container port.
				ContainerPort:      port.Port,
				TargetPort:         &port.TargetPort,
				FailurePolicy:      w.FailurePolicy,
				MatchPolicy:        w.MatchPolicy,
				GenerateName:       w.Name,
				ObjectSelector:     w.ObjectSelector,
				SideEffects:        w.SideEffects,
				TimeoutSeconds:     w.TimeoutSeconds,
				Rules:              w.Rules,
				ConversionCRDs:     nil,
				Type:               operatorsv1alpha1.ValidatingAdmissionWebhook,
				ReinvocationPolicy: nil,
				WebhookPath:        w.ClientConfig.Service.Path,
			})
		}

		delete(unstructuredManifests, p)
	}

	klog.V(4).Infof("Found %d validating webhook(s)", len(validatingWebhookDefinitions))
	csv.Spec.WebhookDefinitions = append(csv.Spec.WebhookDefinitions, validatingWebhookDefinitions...)

	mutatingWebhookConfigurationGVK := admissionregistrationv1.SchemeGroupVersion.WithKind("MutatingWebhookConfiguration")
	var mutatingWebhookDefinitions []operatorsv1alpha1.WebhookDescription
	for p, u := range unstructuredManifests {
		if u.GroupVersionKind() != mutatingWebhookConfigurationGVK {
			continue
		}

		mutatingWebhookConfiguration := &admissionregistrationv1.MutatingWebhookConfiguration{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), mutatingWebhookConfiguration)
		if err != nil {
			return fmt.Errorf("can't convert %T to %T: %w", u, mutatingWebhookConfiguration, err)
		}

		for i, w := range mutatingWebhookConfiguration.Webhooks {
			if w.ClientConfig.Service == nil {
				return fmt.Errorf("file %q: olm supports only service webhhoks", p)
			}

			svcKey := naming.ObjRef(&metav1.ObjectMeta{
				Namespace: w.ClientConfig.Service.Namespace,
				Name:      w.ClientConfig.Service.Name,
			})
			svcWithPath, found := servicesWithPath[svcKey]
			if !found {
				return fmt.Errorf(
					"file %q: MutatingWebhookConfiguration %q webhook %d references unknown service %q",
					p,
					naming.ObjRef(mutatingWebhookConfiguration),
					i,
					svcKey,
				)
			}

			delete(unstructuredManifests, svcWithPath.path)

			d, err := findDeploymentForService(svcWithPath.service, deployments)
			if err != nil {
				return fmt.Errorf("file %q: can't find deployment from service: %w", p, err)
			}

			webhookPort := int32(443)
			if w.ClientConfig.Service.Port != nil {
				webhookPort = *w.ClientConfig.Service.Port
			}

			port, err := findServicePort(svcWithPath.service, webhookPort)
			if err != nil {
				return fmt.Errorf("file %q: can't find deployment for service: %w", p, err)
			}

			mutatingWebhookDefinitions = append(mutatingWebhookDefinitions, operatorsv1alpha1.WebhookDescription{
				DeploymentName:          d.Name,
				AdmissionReviewVersions: w.AdmissionReviewVersions,
				// ContainerPort in OLM actually means the service port,
				// not the target port which is equivalent to container port.
				ContainerPort:      port.Port,
				TargetPort:         &port.TargetPort,
				FailurePolicy:      w.FailurePolicy,
				MatchPolicy:        w.MatchPolicy,
				GenerateName:       w.Name,
				ObjectSelector:     w.ObjectSelector,
				SideEffects:        w.SideEffects,
				TimeoutSeconds:     w.TimeoutSeconds,
				Rules:              w.Rules,
				ConversionCRDs:     nil,
				Type:               operatorsv1alpha1.MutatingAdmissionWebhook,
				ReinvocationPolicy: w.ReinvocationPolicy,
				WebhookPath:        w.ClientConfig.Service.Path,
			})
		}

		delete(unstructuredManifests, p)
	}

	klog.V(4).Infof("Found %d mutating webhook(s)", len(mutatingWebhookDefinitions))
	csv.Spec.WebhookDefinitions = append(csv.Spec.WebhookDefinitions, mutatingWebhookDefinitions...)

	// All transformations have succeeded, we can start writing down files.
	klog.Infof("Transformation are complete, writing down files...")

	// Make sure dirs are in place.
	manifestsDir := path.Join(o.OutputDir, "manifests")
	for _, d := range []string{
		manifestsDir,
	} {
		klog.V(2).Infof("Ensuring directory %q", d)
		err = os.Mkdir(d, 0o777)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("can't create dir %q: %w", d, err)
		}
	}

	fileMap := map[string][]byte{}

	// Marshal CSV.
	csvBytes, err := yaml.Marshal(csv)
	if err != nil {
		return fmt.Errorf("can't marshal csv: %w", err)
	}

	csvFileName := path.Base(o.CSVPath)
	csvOutputPath := path.Join(manifestsDir, csvFileName)
	fileMap[csvOutputPath] = csvBytes

	// Marshal remaining manifests.
	for filePath, unstructuredManifest := range unstructuredManifests {
		jsonBytes, err := unstructuredManifest.MarshalJSON()
		if err != nil {
			return fmt.Errorf("can't marshal %q into json: %w", filePath, err)
		}

		yamlBytes, err := yaml.JSONToYAML(jsonBytes)
		if err != nil {
			return fmt.Errorf("can't convert JSON to YAML for manifest %q: %w", filePath, err)
		}

		outputName := filepath.Base(filePath)
		outputPath := path.Join(manifestsDir, outputName)
		_, found := fileMap[outputPath]
		if found {
			return fmt.Errorf("output manifest path %q conflicts with another manifest", outputPath)
		}

		fileMap[outputPath] = yamlBytes
	}

	// Write down files.
	for filePath, bytes := range fileMap {
		klog.V(2).Infof("Writing down file %q", filePath)
		err := ioutil.WriteFile(filePath, bytes, 0o666)
		if err != nil {
			return fmt.Errorf("can't write csv: %w", err)
		}
	}

	klog.Infof("All files have been written down to the output dir")

	return nil
}

func objLess(lhs, rhs metav1.Object) bool {
	return naming.ObjRef(lhs) < naming.ObjRef(rhs)
}

func findDeploymentForService(svc *corev1.Service, deployments []*appsv1.Deployment) (*appsv1.Deployment, error) {
	var res []*appsv1.Deployment
	for _, d := range deployments {
		if labels.SelectorFromSet(svc.Spec.Selector).Matches(labels.Set(d.Labels)) {
			res = append(res, d)
		}
	}

	switch len(res) {
	case 0:
		return nil, fmt.Errorf("can't find deployment for service %q", naming.ObjRef(svc))
	case 1:
		break
	default:
		var objKeys []string
		for _, d := range res {
			objKeys = append(objKeys, naming.ObjRef(d))
		}
		return nil, fmt.Errorf("service %q is selecting more then 1 deployment: %s", naming.ObjRef(svc), strings.Join(objKeys, ", "))

	}

	return res[0], nil
}

func findServicePort(svc *corev1.Service, port int32) (*corev1.ServicePort, error) {
	for _, p := range svc.Spec.Ports {
		if p.Port == port {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("can't find port %d in service %q", port, naming.ObjRef(svc))
}
