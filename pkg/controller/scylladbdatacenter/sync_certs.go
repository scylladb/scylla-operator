package scylladbdatacenter

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	okubecrypto "github.com/scylladb/scylla-operator/pkg/kubecrypto"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	cqlclientv1alpha1 "github.com/scylladb/scylla-operator/pkg/scylla/api/cqlclient/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/errors"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog/v2"
)

func getHostPort(host string, port int) string {
	if port == 0 {
		return host
	}

	return fmt.Sprintf("%s:%d", host, port)
}

func makeScyllaConnectionConfig(
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	secrets map[string]*corev1.Secret,
	configMaps map[string]*corev1.ConfigMap,
	cqlsIngressPort int,
) (*corev1.Secret, error) {
	clientCertSecretName := naming.GetScyllaClusterLocalUserAdminCertName(sdc.Name)
	clientCertSecret, found := secrets[clientCertSecretName]
	if !found {
		return nil, fmt.Errorf("secret %q doesn't exist or is not own by this object", naming.ManualRef(sdc.Namespace, clientCertSecretName))
	}

	clientCertsBytes, clientKeyBytes, err := okubecrypto.GetCertKeyDataFromSecret(clientCertSecret)
	if err != nil {
		return nil, fmt.Errorf("can't get cert and key bytes from secret %q: %w", clientCertSecretName, err)
	}

	servingCAConfigMapName := naming.GetScyllaClusterLocalServingCAName(sdc.Name)
	servingCAConfigMap, found := configMaps[servingCAConfigMapName]
	if !found {
		return nil, fmt.Errorf("configmap %q doesn't exist or is not own by this object", naming.ManualRef(sdc.Namespace, servingCAConfigMapName))
	}

	servingCABytes, err := okubecrypto.GetCABundleDataFromConfigMap(servingCAConfigMap)
	if err != nil {
		return nil, fmt.Errorf("can't get ca bundle bytes from configmap %q: %w", servingCAConfigMapName, err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: sdc.Namespace,
			Name:      naming.GetScyllaClusterLocalAdminCQLConnectionConfigsName(sdc.Name),
			Labels:    naming.ClusterLabels(sdc),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
			},
		},
		Data: make(map[string][]byte, len(sdc.Spec.DNSDomains)),
		Type: corev1.SecretTypeOpaque,
	}

	for _, domain := range sdc.Spec.DNSDomains {
		scyllaConnectionConfig := &cqlclientv1alpha1.CQLConnectionConfig{
			AuthInfos: map[string]*cqlclientv1alpha1.AuthInfo{
				"admin": {
					ClientCertificateData: clientCertsBytes,
					ClientKeyData:         clientKeyBytes,
					Username:              "cassandra",
					Password:              "cassandra",
				},
			},
			Datacenters: map[string]*cqlclientv1alpha1.Datacenter{
				naming.GetScyllaDBDatacenterGossipDatacenterName(sdc): {
					Server:                   getHostPort(naming.GetCQLProtocolSubDomain(domain), cqlsIngressPort),
					NodeDomain:               naming.GetCQLProtocolSubDomain(domain),
					CertificateAuthorityData: servingCABytes,
				},
			},
			Contexts: map[string]*cqlclientv1alpha1.Context{
				"default": {
					AuthInfoName:   "admin",
					DatacenterName: naming.GetScyllaDBDatacenterGossipDatacenterName(sdc),
				},
			},
			CurrentContext: "default",
			Parameters: &cqlclientv1alpha1.CQLParameters{
				DefaultConsistency:       cqlclientv1alpha1.CQLDefaultQuorumConsistency,
				DefaultSerialConsistency: cqlclientv1alpha1.CQLDefaultSerialConsistency,
			},
		}

		encoder := scheme.Codecs.EncoderForVersion(scheme.DefaultYamlSerializer, schema.GroupVersions(scheme.Scheme.PrioritizedVersionsAllGroups()))
		scyllaConnectionConfigData, err := runtime.Encode(encoder, scyllaConnectionConfig)
		if err != nil {
			return nil, fmt.Errorf("can't encode scylla connection config: %w", err)
		}

		secret.Data[domain] = scyllaConnectionConfigData
	}

	return secret, nil
}

func (sdcc *Controller) syncCerts(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	secrets map[string]*corev1.Secret,
	configMaps map[string]*corev1.ConfigMap,
	serviceMap map[string]*corev1.Service,
) ([]metav1.Condition, error) {
	// Be careful to always apply signers first to be sure they have been persisted
	// before applying any child certificate signed by it.

	var err error
	var errs []error
	var progressingConditions []metav1.Condition

	cm := okubecrypto.NewCertificateManager(
		sdcc.keyGetter,
		sdcc.kubeClient.CoreV1(),
		sdcc.secretLister,
		sdcc.kubeClient.CoreV1(),
		sdcc.configMapLister,
		sdcc.eventRecorder,
	)

	clusterLabels := naming.ClusterLabels(sdc)

	if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
		// Manage client certificates.
		errs = append(errs, cm.ManageCertificates(
			ctx,
			time.Now,
			&sdc.ObjectMeta,
			scyllav1alpha1.ScyllaDBDatacenterGVK,
			&okubecrypto.CAConfig{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   naming.GetScyllaClusterLocalClientCAName(sdc.Name),
					Labels: clusterLabels,
				},
				Validity: 10 * 365 * 24 * time.Hour,
				Refresh:  8 * 365 * 24 * time.Hour,
			},
			&okubecrypto.CABundleConfig{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   naming.GetScyllaClusterLocalClientCAName(sdc.Name),
					Labels: clusterLabels,
				},
			},
			[]*okubecrypto.CertificateConfig{
				{
					MetaConfig: okubecrypto.MetaConfig{
						Name:   naming.GetScyllaClusterLocalUserAdminCertName(sdc.Name),
						Labels: clusterLabels,
					},
					Validity: 10 * 365 * 24 * time.Hour,
					Refresh:  8 * 365 * 24 * time.Hour,
					CertCreator: (&ocrypto.ClientCertCreatorConfig{
						Subject: pkix.Name{
							CommonName: "",
						},
						DNSNames: []string{"admin"},
					}).ToCreator(),
				},
			},
			secrets,
			configMaps,
		))
	}

	// Manage serving certificates.
	// We are currently using StatefulSet that allows only a uniform config so we have to create a multi-SAN certificate.
	// TODO: Create dedicated certs when we get rid of StatefulSets.
	// TODO: we need to somehow check the domain given so we don't start signing stuff for e.g. google.com.
	//       (maybe cluster domains CR, or forcing a suffix). On the other hand the CA should be trusted
	//       only for the connection, not by the system.

	// For the clients not using the proxy we'll sign for IPs that are published by ScyllaDB.
	// We should also sign the identity service ClusterIP and internal DNS for the
	// initial discovery. (The initial contact point is especially important when using ephemeral IPs.)
	var ipAddresses []net.IP
	var servingDNSNames []string

	if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) || sdc.Spec.ScyllaDB.AlternatorOptions != nil {
		for _, svc := range serviceMap {
			svcType := svc.Labels[naming.ScyllaServiceTypeLabel]
			if svcType != string(naming.ScyllaServiceTypeMember) && svcType != string(naming.ScyllaServiceTypeIdentity) {
				continue
			}

			if svc.Spec.ClusterIP != corev1.ClusterIPNone {
				parsedIP, err := helpers.ParseIP(svc.Spec.ClusterIP)
				if err != nil {
					return progressingConditions, fmt.Errorf("can't parse Service %q ClusterIP %q: %w", naming.ObjRef(svc), svc.Spec.ClusterIP, err)
				}

				ipAddresses = append(ipAddresses, parsedIP)
			}

			if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
				hasExternalAddress := false
				for _, ingressStatus := range svc.Status.LoadBalancer.Ingress {
					if len(ingressStatus.IP) != 0 {
						parsedIP := net.ParseIP(ingressStatus.IP)
						if parsedIP == nil {
							return progressingConditions, fmt.Errorf("can't parse ingress IP %q", ingressStatus.IP)
						}

						hasExternalAddress = true
						ipAddresses = append(ipAddresses, parsedIP)
					}

					if len(ingressStatus.Hostname) != 0 {
						hasExternalAddress = true
						servingDNSNames = append(servingDNSNames, ingressStatus.Hostname)
					}
				}

				// There should be at least one address for ServiceTypeLoadBalancer.
				if !hasExternalAddress {
					progressingConditions = append(progressingConditions, metav1.Condition{
						Type:               certControllerProgressingCondition,
						Status:             metav1.ConditionTrue,
						Reason:             internalapi.ProgressingReason,
						Message:            fmt.Sprintf("waiting for external address in Service %q to be available", naming.ObjRef(svc)),
						ObservedGeneration: sdc.Generation,
					})
				}
			}

			// Only member services have associated pods
			if svcType != string(naming.ScyllaServiceTypeMember) {
				continue
			}

			pod, err := sdcc.podLister.Pods(sdc.Namespace).Get(svc.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					progressingConditions = append(progressingConditions, metav1.Condition{
						Type:               certControllerProgressingCondition,
						Status:             metav1.ConditionTrue,
						Reason:             internalapi.ProgressingReason,
						Message:            fmt.Sprintf("waiting for Pod %q to be created", naming.ManualRef(sdc.Namespace, svc.Name)),
						ObservedGeneration: sdc.Generation,
					})
					continue
				}

				return progressingConditions, fmt.Errorf("can't get Pod %q: %w", naming.ManualRef(sdc.Namespace, svc.Name), err)
			}

			if len(pod.Status.PodIP) == 0 {
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               certControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             internalapi.ProgressingReason,
					Message:            fmt.Sprintf("waiting for Pod %q to have an IP address", naming.ManualRef(sdc.Namespace, svc.Name)),
					ObservedGeneration: sdc.Generation,
				})
				continue
			}

			parsedIP := net.ParseIP(pod.Status.PodIP)
			if parsedIP == nil {
				return progressingConditions, fmt.Errorf("can't parse Pod %q IP %q", naming.ObjRef(pod), pod.Status.PodIP)
			}

			ipAddresses = append(ipAddresses, parsedIP)
		}

		// Make sure ipAddresses are always sorted and can be reconciled in a declarative way.
		sort.SliceStable(ipAddresses, func(i, j int) bool {
			return ipAddresses[i].String() < ipAddresses[j].String()
		})

		var hostIDs []string
		var progressingMessages []string
		for _, svc := range serviceMap {
			if svc.Labels[naming.ScyllaServiceTypeLabel] != string(naming.ScyllaServiceTypeMember) {
				continue
			}

			hostID, hasHostID := svc.Annotations[naming.HostIDAnnotation]
			if len(hostID) == 0 {
				if hasHostID {
					klog.Warningf("service %q has empty hostID", klog.KObj(svc))
				}

				message := fmt.Sprintf("waiting for service %q to have a hostID set", naming.ObjRef(svc))
				progressingMessages = append(progressingMessages, message)

				continue
			}

			hostIDs = append(hostIDs, hostID)
		}

		// For every cluster domain we'll create "cql" subdomain and "UUID.cql" subdomains for every node.
		for _, domain := range sdc.Spec.DNSDomains {
			servingDNSNames = append(servingDNSNames, naming.GetCQLProtocolSubDomain(domain))

			for _, hostID := range hostIDs {
				servingDNSNames = append(servingDNSNames, naming.GetCQLHostIDSubDomain(hostID, domain))
			}
		}

		// Sign for every node DNS name and discovery endpoint.
		for _, svc := range serviceMap {
			svcType := svc.Labels[naming.ScyllaServiceTypeLabel]
			if svcType != string(naming.ScyllaServiceTypeIdentity) && svcType != string(naming.ScyllaServiceTypeMember) {
				return progressingConditions, fmt.Errorf("can't sign certificate for DNS name of unknown service type %q", svcType)
			}
			servingDNSNames = append(servingDNSNames, fmt.Sprintf("%s.%s.svc", svc.Name, svc.Namespace))
		}

		// Make sure servingDNSNames are always sorted and can be reconciled in a declarative way.
		sort.Strings(servingDNSNames)

		if len(progressingMessages) != 0 {
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               certControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             internalapi.ProgressingReason,
				Message:            strings.Join(progressingMessages, "\n"),
				ObservedGeneration: sdc.Generation,
			})
		}
	}

	if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
		errs = append(errs, cm.ManageCertificates(
			ctx,
			time.Now,
			&sdc.ObjectMeta,
			scyllav1alpha1.ScyllaDBDatacenterGVK,
			&okubecrypto.CAConfig{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   naming.GetScyllaClusterLocalServingCAName(sdc.Name),
					Labels: clusterLabels,
				},
				Validity: 10 * 365 * 24 * time.Hour,
				Refresh:  8 * 365 * 24 * time.Hour,
			},
			&okubecrypto.CABundleConfig{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   naming.GetScyllaClusterLocalServingCAName(sdc.Name),
					Labels: clusterLabels,
				},
			},
			[]*okubecrypto.CertificateConfig{
				{
					MetaConfig: okubecrypto.MetaConfig{
						Name:   naming.GetScyllaClusterLocalServingCertName(sdc.Name),
						Labels: clusterLabels,
					},
					Validity: 30 * 24 * time.Hour,
					Refresh:  20 * 24 * time.Hour,
					CertCreator: (&ocrypto.ServingCertCreatorConfig{
						Subject: pkix.Name{
							CommonName: "",
						},
						IPAddresses: ipAddresses,
						DNSNames:    servingDNSNames,
					}).ToCreator(),
				},
			},
			secrets,
			configMaps,
		))

		// Build connection bundle.

		scyllaConnectionConfigSecret, err := makeScyllaConnectionConfig(sdc, secrets, configMaps, sdcc.cqlsIngressPort)
		if err != nil {
			errs = append(errs, err)
		} else {
			_, changed, err := resourceapply.ApplySecret(ctx, sdcc.kubeClient.CoreV1(), sdcc.secretLister, sdcc.eventRecorder, scyllaConnectionConfigSecret, resourceapply.ApplyOptions{})
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, certControllerProgressingCondition, scyllaConnectionConfigSecret, "apply", sdc.Generation)
			}
			if err != nil {
				return progressingConditions, fmt.Errorf("can't apply secret %q: %w", naming.ObjRef(scyllaConnectionConfigSecret), err)
			}
		}
	}

	// Setup Alternator certificates.
	if sdc.Spec.ScyllaDB.AlternatorOptions != nil &&
		(sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate == nil || sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.Type == scyllav1alpha1.TLSCertificateTypeOperatorManaged) {
		var additionalDNSNames []string
		if sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.OperatorManagedOptions != nil && sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.OperatorManagedOptions.AdditionalDNSNames != nil {
			additionalDNSNames = sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.OperatorManagedOptions.AdditionalDNSNames
		}
		alternatorDNSNames := make([]string, 0, len(servingDNSNames)+len(additionalDNSNames))
		alternatorDNSNames = append(alternatorDNSNames, servingDNSNames...)
		alternatorDNSNames = append(alternatorDNSNames, additionalDNSNames...)

		var additionalIPAddresses []net.IP
		if sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.OperatorManagedOptions != nil && sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.OperatorManagedOptions.AdditionalIPAddresses != nil {
			additionalIPAddresses, err = helpers.ParseIPs(sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.OperatorManagedOptions.AdditionalIPAddresses)
			if err != nil {
				return nil, fmt.Errorf("can't parse additional IP addresses for alternator serving cert: %w", err)
			}
		}
		alternatorIPAddresses := make([]net.IP, 0, len(ipAddresses)+len(additionalIPAddresses))
		alternatorIPAddresses = append(alternatorIPAddresses, ipAddresses...)
		alternatorIPAddresses = append(alternatorIPAddresses, additionalIPAddresses...)

		errs = append(errs, cm.ManageCertificates(
			ctx,
			time.Now,
			&sdc.ObjectMeta,
			scyllav1alpha1.ScyllaDBDatacenterGVK,
			&okubecrypto.CAConfig{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   naming.GetScyllaClusterAlternatorLocalServingCAName(sdc.Name),
					Labels: clusterLabels,
				},
				Validity: 10 * 365 * 24 * time.Hour,
				Refresh:  8 * 365 * 24 * time.Hour,
			},
			&okubecrypto.CABundleConfig{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   naming.GetScyllaClusterAlternatorLocalServingCAName(sdc.Name),
					Labels: clusterLabels,
				},
			},
			[]*okubecrypto.CertificateConfig{
				{
					MetaConfig: okubecrypto.MetaConfig{
						Name:   naming.GetScyllaClusterAlternatorLocalServingCertName(sdc.Name),
						Labels: clusterLabels,
					},
					Validity: 30 * 24 * time.Hour,
					Refresh:  20 * 24 * time.Hour,
					CertCreator: (&ocrypto.ServingCertCreatorConfig{
						Subject: pkix.Name{
							CommonName: "",
						},
						IPAddresses: alternatorIPAddresses,
						DNSNames:    alternatorDNSNames,
					}).ToCreator(),
				},
			},
			secrets,
			configMaps,
		))
	}

	return progressingConditions, errors.NewAggregate(errs)
}
