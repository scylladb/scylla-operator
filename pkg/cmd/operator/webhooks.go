package operator

import (
	"bytes"
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
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/admissionreview"
	"github.com/scylladb/scylla-operator/pkg/api/conversion"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	validationv1 "github.com/scylladb/scylla-operator/pkg/api/validation/v1"
	validationv1alpha1 "github.com/scylladb/scylla-operator/pkg/api/validation/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/conversionreview"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apijson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type WebhookOptions struct {
	TLSCertFile, TLSKeyFile        string
	Port                           int
	InsecureGenerateLocalhostCerts bool

	TLSConfig                 *tls.Config
	dynamicCertKeyPairContent *dynamiccertificates.DynamicCertKeyPairContent

	resolvedListenAddr   string
	resolvedListenAddrCh chan struct{}
}

func NewWebhookOptions(streams genericclioptions.IOStreams) *WebhookOptions {
	return &WebhookOptions{
		Port:                 5000,
		resolvedListenAddrCh: make(chan struct{}),
	}
}

func NewWebhookCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewWebhookOptions(streams)

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
	cmd.Flags().IntVarP(&o.Port, "port", "", o.Port, "Secure port that the webhook listens on.")

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

	return utilerrors.NewAggregate(errs)
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
	klog.Infof("%s version %s", cmd.Name(), version.Get())
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
	handler.Handle("/validate", admissionreview.NewHandler(validate))
	handler.Handle("/convert", conversionreview.NewHandler(convert))

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

func validate(ar *admissionv1.AdmissionReview) error {
	gvr := schema.GroupVersionResource{
		Group:    ar.Request.Resource.Group,
		Version:  ar.Request.Resource.Version,
		Resource: ar.Request.Resource.Resource,
	}

	deserializer := codecs.UniversalDeserializer()

	var err error
	var obj, oldObj runtime.Object
	if ar.Request.Object.Raw != nil {
		obj, _, err = deserializer.Decode(ar.Request.Object.Raw, nil, nil)
		if err != nil {
			return fmt.Errorf("can't decode object %q: %w", gvr, err)
		}
	}
	if ar.Request.OldObject.Raw != nil {
		oldObj, _, err = deserializer.Decode(ar.Request.OldObject.Raw, nil, nil)
		if err != nil {
			return fmt.Errorf("can't decode old object %q: %w", gvr, err)
		}
	}

	switch gvr {
	case scyllav1alpha1.GroupVersion.WithResource("scylladatacenters"):
		var errList field.ErrorList
		switch ar.Request.Operation {
		case admissionv1.Create:
			errList = validationv1alpha1.ValidateScyllaDatacenter(obj.(*scyllav1alpha1.ScyllaDatacenter))
		case admissionv1.Update:
			errList = validationv1alpha1.ValidateScyllaDatacenterUpdate(obj.(*scyllav1alpha1.ScyllaDatacenter), oldObj.(*scyllav1alpha1.ScyllaDatacenter))
		}

		if len(errList) > 0 {
			return apierrors.NewInvalid(obj.(*scyllav1alpha1.ScyllaDatacenter).GroupVersionKind().GroupKind(), obj.(*scyllav1alpha1.ScyllaDatacenter).Name, errList)
		}
		return nil
	case scyllav1.GroupVersion.WithResource("scyllaclusters"):
		var errList field.ErrorList
		switch ar.Request.Operation {
		case admissionv1.Create:
			errList = validationv1.ValidateScyllaCluster(obj.(*scyllav1.ScyllaCluster))
		case admissionv1.Update:
			errList = validationv1.ValidateScyllaClusterUpdate(obj.(*scyllav1.ScyllaCluster), oldObj.(*scyllav1.ScyllaCluster))
		}

		if len(errList) > 0 {
			return apierrors.NewInvalid(obj.(*scyllav1.ScyllaCluster).GroupVersionKind().GroupKind(), obj.(*scyllav1.ScyllaCluster).Name, errList)
		}
		return nil
	default:
		return fmt.Errorf("unsupported GVR %q", gvr)
	}
}

func convert(cr *apiextensionsv1.ConversionReview) error {
	deserializer := codecs.UniversalDeserializer()
	serializer := apijson.NewSerializerWithOptions(apijson.DefaultMetaFactory, scheme, scheme, apijson.SerializerOptions{})

	desiredGV, err := schema.ParseGroupVersion(cr.Request.DesiredAPIVersion)
	if err != nil {
		return fmt.Errorf("can't parse desired group version %q: %w", cr.Request.DesiredAPIVersion, err)
	}

	encoder := codecs.EncoderForVersion(serializer, desiredGV)

	var errs []error
	buf := bytes.Buffer{}

	for i, obj := range cr.Request.Objects {
		decodedObject, _, err := deserializer.Decode(obj.Raw, nil, nil)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't decode %d'th object: %w", i, err))
			continue
		}
		gvk := decodedObject.GetObjectKind().GroupVersionKind()

		klog.Infof("Converting %q into %q", gvk, desiredGV)

		var convertedObj runtime.Object

		switch gvk.Kind {
		case "ScyllaCluster":
			switch cr.Request.DesiredAPIVersion {
			case scyllav2alpha1.GroupVersion.String():
				switch gvk.Version {
				case "v1":
					convertedObj, err = conversion.ScyllaClusterFromV1ToV2Alpha1(decodedObject.(*scyllav1.ScyllaCluster))
					if err != nil {
						errs = append(errs, fmt.Errorf("can't convert %d'ith obj to v2alpha1.ScyllaCluster: %w", i, err))
						continue
					}
				default:
					return fmt.Errorf("unsupported conversion from %q to %q", gvk.Version, cr.Request.DesiredAPIVersion)
				}
			case scyllav1.GroupVersion.String():
				switch gvk.Version {
				case "v2alpha1":
					convertedObj, err = conversion.ScyllaClusterFromV2Alpha1oV1(decodedObject.(*scyllav2alpha1.ScyllaCluster))
					if err != nil {
						errs = append(errs, fmt.Errorf("can't convert %d'ith obj to v1.ScyllaCluster: %w", i, err))
						continue
					}
				default:
					return fmt.Errorf("unsupported conversion from %q to %q", gvk.Version, cr.Request.DesiredAPIVersion)
				}
			default:
				return fmt.Errorf("unsupported desired API version %q", cr.Request.DesiredAPIVersion)
			}
		default:
			return fmt.Errorf("unsupported GVK %q", gvk)
		}

		buf.Reset()
		if err := encoder.Encode(convertedObj, &buf); err != nil {
			errs = append(errs, fmt.Errorf("can't convert to desired apiVersion: %v", err))
			continue
		}

		cr.Response.ConvertedObjects = append(cr.Response.ConvertedObjects, runtime.RawExtension{Raw: buf.Bytes()})
	}

	return utilerrors.NewAggregate(errs)
}
