package operator

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/admissionreview"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/validation"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/spf13/cobra"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

var (
	DefaultValidators = map[schema.GroupVersionResource]Validator{
		scyllav1.GroupVersion.WithResource("scyllaclusters"): &GenericValidator[*scyllav1.ScyllaCluster]{
			ValidateCreateFunc:      validation.ValidateScyllaCluster,
			ValidateUpdateFunc:      validation.ValidateScyllaClusterUpdate,
			GetWarningsOnCreateFunc: validation.GetWarningsOnScyllaClusterCreate,
			GetWarningsOnUpdateFunc: validation.GetWarningsOnScyllaClusterUpdate,
		},
		scyllav1alpha1.GroupVersion.WithResource("nodeconfigs"): &GenericValidator[*scyllav1alpha1.NodeConfig]{
			ValidateCreateFunc:      validation.ValidateNodeConfig,
			ValidateUpdateFunc:      validation.ValidateNodeConfigUpdate,
			GetWarningsOnCreateFunc: validation.GetWarningsOnNodeConfigCreate,
			GetWarningsOnUpdateFunc: validation.GetWarningsOnNodeConfigUpdate,
		},
		scyllav1alpha1.GroupVersion.WithResource("scyllaoperatorconfigs"): &GenericValidator[*scyllav1alpha1.ScyllaOperatorConfig]{
			ValidateCreateFunc:      validation.ValidateScyllaOperatorConfig,
			ValidateUpdateFunc:      validation.ValidateScyllaOperatorConfigUpdate,
			GetWarningsOnCreateFunc: validation.GetWarningsOnScyllaOperatorConfigCreate,
			GetWarningsOnUpdateFunc: validation.GetWarningsOnScyllaOperatorConfigUpdate,
		},
		scyllav1alpha1.GroupVersion.WithResource("scylladbdatacenters"): &GenericValidator[*scyllav1alpha1.ScyllaDBDatacenter]{
			ValidateCreateFunc:      validation.ValidateScyllaDBDatacenter,
			ValidateUpdateFunc:      validation.ValidateScyllaDBDatacenterUpdate,
			GetWarningsOnCreateFunc: validation.GetWarningsOnScyllaDBDatacenterCreate,
			GetWarningsOnUpdateFunc: validation.GetWarningsOnScyllaDBDatacenterUpdate,
		},
		scyllav1alpha1.GroupVersion.WithResource("scylladbclusters"): &GenericValidator[*scyllav1alpha1.ScyllaDBCluster]{
			ValidateCreateFunc:      validation.ValidateScyllaDBCluster,
			ValidateUpdateFunc:      validation.ValidateScyllaDBClusterUpdate,
			GetWarningsOnCreateFunc: validation.GetWarningsOnScyllaDBClusterCreate,
			GetWarningsOnUpdateFunc: validation.GetWarningsOnScyllaDBClusterUpdate,
		},
		scyllav1alpha1.GroupVersion.WithResource("scylladbmanagerclusterregistrations"): &GenericValidator[*scyllav1alpha1.ScyllaDBManagerClusterRegistration]{
			ValidateCreateFunc:      validation.ValidateScyllaDBManagerClusterRegistration,
			ValidateUpdateFunc:      validation.ValidateScyllaDBManagerClusterRegistrationUpdate,
			GetWarningsOnCreateFunc: validation.GetWarningsOnScyllaDBManagerClusterRegistrationCreate,
			GetWarningsOnUpdateFunc: validation.GetWarningsOnScyllaDBManagerClusterRegistrationUpdate,
		},
		scyllav1alpha1.GroupVersion.WithResource("scylladbmanagertasks"): &GenericValidator[*scyllav1alpha1.ScyllaDBManagerTask]{
			ValidateCreateFunc:      validation.ValidateScyllaDBManagerTask,
			ValidateUpdateFunc:      validation.ValidateScyllaDBManagerTaskUpdate,
			GetWarningsOnCreateFunc: validation.GetWarningsOnScyllaDBManagerTaskCreate,
			GetWarningsOnUpdateFunc: validation.GetWarningsOnScyllaDBManagerTaskUpdate,
		},
		scyllav1alpha1.GroupVersion.WithResource("scylladbmonitorings"): &GenericValidator[*scyllav1alpha1.ScyllaDBMonitoring]{
			ValidateCreateFunc:      validation.ValidateScyllaDBMonitoring,
			ValidateUpdateFunc:      validation.ValidateScyllaDBMonitoringUpdate,
			GetWarningsOnCreateFunc: validation.GetWarningsOnScyllaDBMonitoringCreate,
			GetWarningsOnUpdateFunc: validation.GetWarningsOnScyllaDBMonitoringUpdate,
		},
	}
)

type Validator interface {
	ValidateCreate(obj runtime.Object) field.ErrorList
	ValidateUpdate(obj, oldObj runtime.Object) field.ErrorList
	GetGroupKind(obj runtime.Object) schema.GroupKind
	GetName(obj runtime.Object) string
	GetWarningsOnCreate(obj runtime.Object) []string
	GetWarningsOnUpdate(obj, oldObj runtime.Object) []string
}

// Container runtime may set env var (SCYLLA_OPERATOR_PORT) to either number or "tcp://<service-host>:<service-port>"
var portFlagRe = regexp.MustCompile(`^(?:tcp://[^:]+:)?(\d+)$`)

type portFlag int

func (pf *portFlag) String() string {
	return fmt.Sprintf("%d", pf)
}

func (pf *portFlag) Set(v string) error {
	matches := portFlagRe.FindStringSubmatch(v)
	if len(matches) < 2 {
		return fmt.Errorf("invalid port format %q", v)
	}

	port, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid port %q", matches[1])
	}

	*pf = portFlag(port)
	return nil
}

func (pf *portFlag) Type() string {
	return "int"
}

type WebhookOptions struct {
	TLSCertFile, TLSKeyFile        string
	Port                           portFlag
	InsecureGenerateLocalhostCerts bool

	Validators map[schema.GroupVersionResource]Validator

	TLSConfig                 *tls.Config
	dynamicCertKeyPairContent *dynamiccertificates.DynamicCertKeyPairContent

	resolvedListenAddr   string
	resolvedListenAddrCh chan struct{}
}

func NewWebhookOptions(streams genericclioptions.IOStreams, validators map[schema.GroupVersionResource]Validator) *WebhookOptions {
	if len(validators) == 0 {
		panic(fmt.Errorf("validators must not be empty"))
	}

	return &WebhookOptions{
		Port:                 5000,
		Validators:           validators,
		resolvedListenAddrCh: make(chan struct{}),
	}
}

func NewWebhookCmd(streams genericclioptions.IOStreams, validators map[schema.GroupVersionResource]Validator) *cobra.Command {
	o := NewWebhookOptions(streams, validators)

	cmd := &cobra.Command{
		Use:   "run-webhook-server",
		Short: "Run webhook server.",
		Long:  "Run webhook server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd)
			if err != nil {
				return err
			}

			return nil
		},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.Flags().StringVarP(&o.TLSCertFile, "tls-cert-file", "", o.TLSCertFile, "File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert).")
	cmd.Flags().StringVarP(&o.TLSKeyFile, "tls-private-key-file", "", o.TLSKeyFile, "File containing the default x509 private key for matching cert file.")
	cmd.Flags().VarP(&o.Port, "port", "", "Secure port that the webhook listens on.")

	cmd.Flags().BoolVarP(&o.InsecureGenerateLocalhostCerts, "insecure-generate-localhost-cert", "", o.InsecureGenerateLocalhostCerts, "This will automatically generate self-signed certificate valid for localhost. Do not use this in production!")
	return cmd
}

func (o *WebhookOptions) Validate() error {
	var errs []error

	if len(o.TLSCertFile) == 0 && !o.InsecureGenerateLocalhostCerts {
		return errors.New("tls-cert-file can't be empty if tls-private-key-file is set")
	}

	if len(o.TLSKeyFile) == 0 && !o.InsecureGenerateLocalhostCerts {
		return errors.New("tls-private-key-file can't be empty if tls-cert-file is set")
	}

	if o.Port == 0 {
		return errors.New("port can't be zero")
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *WebhookOptions) Complete() error {
	var err error

	if o.InsecureGenerateLocalhostCerts {
		klog.Warningf("Generating temporary TLS certificate.")

		privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return err
		}

		serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
		serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
		if err != nil {
			return err
		}

		now := time.Now()

		template := x509.Certificate{
			SerialNumber: serialNumber,
			NotBefore:    now,
			NotAfter:     now.Add(24 * time.Hour),

			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
			IsCA:                  true,

			DNSNames: []string{"localhost"},
		}
		derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, privateKey.Public(), privateKey)
		if err != nil {
			return err
		}

		o.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{
				{
					Certificate: [][]byte{derBytes},
					PrivateKey:  privateKey,
				},
			},
		}
	} else {
		o.dynamicCertKeyPairContent, err = dynamiccertificates.NewDynamicServingContentFromFiles("serving-certs", o.TLSCertFile, o.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("can't create DynamicServingContentFromFiles: %w", err)
		}

		o.TLSConfig = &tls.Config{
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				cert, err := tls.X509KeyPair(o.dynamicCertKeyPairContent.CurrentCertKeyContent())
				return &cert, err
			},
		}
	}

	return nil
}

func (o *WebhookOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	cmdutil.LogCommandStarting(cmd)
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	return o.run(ctx, streams)
}

func (o *WebhookOptions) run(ctx context.Context, streams genericclioptions.IOStreams) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	handler := http.NewServeMux()
	handler.HandleFunc("/readyz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("ok"))
		if err != nil {
			klog.Error(err)
		}
	})
	handler.Handle("/validate", admissionreview.NewHandler(func(review *admissionv1.AdmissionReview) ([]string, error) {
		return validate(review, o.Validators)
	}))

	server := http.Server{
		Handler:   handler,
		TLSConfig: o.TLSConfig,
	}

	if o.dynamicCertKeyPairContent != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o.dynamicCertKeyPairContent.Run(ctx, 1)
		}()
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", o.Port))
	if err != nil {
		return fmt.Errorf("can't create listener: %w", err)
	}
	defer listener.Close()

	o.resolvedListenAddr = listener.Addr().String()
	// Notify anyone waiting for establishing the listen address by closing the channel.
	close(o.resolvedListenAddrCh)

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()
		klog.Infof("Shutting down the server.")
		err := server.Shutdown(context.Background())
		if err != nil {
			klog.ErrorS(err, "can't shutdown the server")
		}
	}()

	klog.Infof("Starting HTTPS server on address %q.", o.resolvedListenAddr)
	err = server.ServeTLS(listener, "", "")
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

type ValidatableObject interface {
	kubeinterfaces.ObjectInterface
	schema.ObjectKind
}

type GenericValidator[T ValidatableObject] struct {
	ValidateCreateFunc      func(obj T) field.ErrorList
	ValidateUpdateFunc      func(obj, oldObj T) field.ErrorList
	GetWarningsOnCreateFunc func(obj T) []string
	GetWarningsOnUpdateFunc func(obj, oldObj T) []string
}

func (v *GenericValidator[T]) ValidateCreate(obj runtime.Object) field.ErrorList {
	return v.ValidateCreateFunc(obj.(T))
}

func (v *GenericValidator[T]) ValidateUpdate(obj, oldObj runtime.Object) field.ErrorList {
	return v.ValidateUpdateFunc(obj.(T), oldObj.(T))
}

func (v *GenericValidator[T]) GetGroupKind(obj runtime.Object) schema.GroupKind {
	return obj.(T).GroupVersionKind().GroupKind()
}

func (v *GenericValidator[T]) GetName(obj runtime.Object) string {
	return obj.(T).GetName()
}

func (v *GenericValidator[T]) GetWarningsOnCreate(obj runtime.Object) []string {
	return v.GetWarningsOnCreateFunc(obj.(T))
}

func (v *GenericValidator[T]) GetWarningsOnUpdate(obj, oldObj runtime.Object) []string {
	return v.GetWarningsOnUpdateFunc(obj.(T), oldObj.(T))
}

func validate(ar *admissionv1.AdmissionReview, validators map[schema.GroupVersionResource]Validator) ([]string, error) {
	gvr := schema.GroupVersionResource{
		Group:    ar.Request.Resource.Group,
		Version:  ar.Request.Resource.Version,
		Resource: ar.Request.Resource.Resource,
	}

	deserializer := scheme.Codecs.UniversalDeserializer()

	var err error
	var obj, oldObj runtime.Object
	if ar.Request.Object.Raw != nil {
		obj, _, err = deserializer.Decode(ar.Request.Object.Raw, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("can't decode object %q: %w", gvr, err)
		}
	}
	if ar.Request.OldObject.Raw != nil {
		oldObj, _, err = deserializer.Decode(ar.Request.OldObject.Raw, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("can't decode old object %q: %w", gvr, err)
		}
	}

	validator, ok := validators[gvr]
	if !ok {
		return nil, fmt.Errorf("unsupported GVR %q", gvr)
	}

	var errList field.ErrorList
	var warnings []string
	switch ar.Request.Operation {
	case admissionv1.Create:
		errList = validator.ValidateCreate(obj)
		warnings = validator.GetWarningsOnCreate(obj)

	case admissionv1.Update:
		errList = validator.ValidateUpdate(obj, oldObj)
		warnings = validator.GetWarningsOnUpdate(obj, oldObj)

	default:
		return nil, fmt.Errorf("unsupported operation %q", ar.Request.Operation)

	}

	if len(errList) > 0 {
		return warnings, apierrors.NewInvalid(validator.GetGroupKind(obj), validator.GetName(obj), errList)
	}

	return warnings, nil
}
