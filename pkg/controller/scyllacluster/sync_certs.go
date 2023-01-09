package scyllacluster

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	okubecrypto "github.com/scylladb/scylla-operator/pkg/kubecrypto"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	cqlclientv1alpha1 "github.com/scylladb/scylla-operator/pkg/scylla/api/cqlclient/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func getHostPort(host string, port int) string {
	if port == 0 {
		return host
	}

	return fmt.Sprintf("%s:%d", host, port)
}

func makeScyllaConnectionConfig(
	sc *scyllav1.ScyllaCluster,
	secrets map[string]*corev1.Secret,
	configMaps map[string]*corev1.ConfigMap,
	cqlsIngressPort int,
) (*corev1.Secret, error) {
	clientCertSecretName := naming.GetScyllaClusterLocalUserAdminCertName(sc.Name)
	clientCertSecret, found := secrets[clientCertSecretName]
	if !found {
		return nil, fmt.Errorf("secret %q doesn't exist or is not own by this object", naming.ManualRef(sc.Namespace, clientCertSecretName))
	}

	clientCertsBytes, clientKeyBytes, err := okubecrypto.GetCertKeyDataFromSecret(clientCertSecret)
	if err != nil {
		return nil, fmt.Errorf("can't get cert and key bytes from secret %q: %w", clientCertSecretName, err)
	}

	servingCAConfigMapName := naming.GetScyllaClusterLocalServingCAName(sc.Name)
	servingCAConfigMap, found := configMaps[servingCAConfigMapName]
	if !found {
		return nil, fmt.Errorf("configmap %q doesn't exist or is not own by this object", naming.ManualRef(sc.Namespace, servingCAConfigMapName))
	}

	servingCABytes, err := okubecrypto.GetCABundleDataFromConfigMap(servingCAConfigMap)
	if err != nil {
		return nil, fmt.Errorf("can't get ca bundle bytes from configmap %q: %w", servingCAConfigMapName, err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: sc.Namespace,
			Name:      naming.GetScyllaClusterLocalAdminCQLConnectionConfigsName(sc.Name),
			Labels:    naming.ClusterLabels(sc),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sc, scyllaClusterControllerGVK),
			},
		},
		Data: make(map[string][]byte, len(sc.Spec.DNSDomains)),
		Type: corev1.SecretTypeOpaque,
	}

	for _, domain := range sc.Spec.DNSDomains {
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
				sc.Spec.Datacenter.Name: {
					Server:                   getHostPort(naming.GetCQLProtocolSubDomain(domain), cqlsIngressPort),
					NodeDomain:               naming.GetCQLProtocolSubDomain(domain),
					CertificateAuthorityData: servingCABytes,
				},
			},
			Contexts: map[string]*cqlclientv1alpha1.Context{
				"default": {
					AuthInfoName:   "admin",
					DatacenterName: sc.Spec.Datacenter.Name,
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

func (scc *Controller) syncCerts(
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
	secrets map[string]*corev1.Secret,
	configMaps map[string]*corev1.ConfigMap,
	serviceMap map[string]*corev1.Service,
) ([]metav1.Condition, error) {
	// Be careful to always apply signers first to be sure they have been persisted
	// before applying any child certificate signed by it.

	var errors []error
	var progressingConditions []metav1.Condition

	cm := okubecrypto.NewCertificateManager(
		scc.kubeClient.CoreV1(),
		scc.secretLister,
		scc.kubeClient.CoreV1(),
		scc.configMapLister,
		scc.eventRecorder,
	)

	clusterLabels := naming.ClusterLabels(sc)

	// Manage client certificates.
	errors = append(errors, cm.ManageCertificates(
		ctx,
		time.Now,
		&sc.ObjectMeta,
		scyllaClusterControllerGVK,
		&okubecrypto.CAConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   naming.GetScyllaClusterLocalClientCAName(sc.Name),
				Labels: clusterLabels,
			},
			Validity: 10 * 365 * 24 * time.Hour,
			Refresh:  8 * 365 * 24 * time.Hour,
		},
		&okubecrypto.CABundleConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   naming.GetScyllaClusterLocalClientCAName(sc.Name),
				Labels: clusterLabels,
			},
		},
		[]*okubecrypto.CertificateConfig{
			{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   naming.GetScyllaClusterLocalUserAdminCertName(sc.Name),
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

	// Manage serving certificates.
	// We are currently using StatefulSet that allows only a uniform config so we have to create a multi-SAN certificate.
	// TODO: Create dedicated certs when we get rid of StatefulSets.
	// TODO: we need to somehow check the domain given so we don't start signing stuff for e.g. google.com.
	//       (maybe cluster domains CR, or forcing a suffix). On the other hand the CA should be trusted
	//       only for the connection, not by the system.

	// For the clients not using the proxy we'll sign for IPs that are published by ScyllaDB.
	var ipAddresses []net.IP
	for _, svc := range serviceMap {
		if svc.Labels[naming.ScyllaServiceTypeLabel] != string(naming.ScyllaServiceTypeMember) {
			continue
		}

		ips := svc.Spec.ClusterIPs
		if len(ips) == 0 {
			klog.V(2).InfoS("No IP available for member", "Service", klog.KObj(svc))
			continue
		}
		parsedIPs, err := helpers.ParseClusterIPs(ips)
		if err != nil {
			return progressingConditions, err
		}

		ipAddresses = append(ipAddresses, parsedIPs...)
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
	var servingDNSNames []string
	for _, domain := range sc.Spec.DNSDomains {
		servingDNSNames = append(servingDNSNames, naming.GetCQLProtocolSubDomain(domain))

		for _, hostID := range hostIDs {
			servingDNSNames = append(servingDNSNames, naming.GetCQLHostIDSubDomain(hostID, domain))
		}
	}
	// Make sure servingDNSNames are always sorted and can be reconciled in a declarative way.
	sort.Strings(servingDNSNames)

	if len(progressingMessages) != 0 {
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               certControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.ProgressingReason,
			Message:            strings.Join(progressingMessages, "\n"),
			ObservedGeneration: sc.Generation,
		})
	}

	errors = append(errors, cm.ManageCertificates(
		ctx,
		time.Now,
		&sc.ObjectMeta,
		scyllaClusterControllerGVK,
		&okubecrypto.CAConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   naming.GetScyllaClusterLocalServingCAName(sc.Name),
				Labels: clusterLabels,
			},
			Validity: 10 * 365 * 24 * time.Hour,
			Refresh:  8 * 365 * 24 * time.Hour,
		},
		&okubecrypto.CABundleConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   naming.GetScyllaClusterLocalServingCAName(sc.Name),
				Labels: clusterLabels,
			},
		},
		[]*okubecrypto.CertificateConfig{
			{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   naming.GetScyllaClusterLocalServingCertName(sc.Name),
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

	scyllaConnectionConfigSecret, err := makeScyllaConnectionConfig(sc, secrets, configMaps, scc.cqlsIngressPort)
	if err != nil {
		errors = append(errors, err)
	} else {
		_, changed, err := resourceapply.ApplySecret(ctx, scc.kubeClient.CoreV1(), scc.secretLister, scc.eventRecorder, scyllaConnectionConfigSecret, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, certControllerProgressingCondition, scyllaConnectionConfigSecret, "apply", sc.Generation)
		}
		if err != nil {
			return progressingConditions, fmt.Errorf("can't apply secret %q: %w", naming.ObjRef(scyllaConnectionConfigSecret), err)
		}
	}

	return progressingConditions, apierrors.NewAggregate(errors)
}
