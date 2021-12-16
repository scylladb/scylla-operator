package sidecar

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/util/network"
	corev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

type Prober struct {
	namespace     string
	serviceName   string
	secretName    string
	serviceLister corev1.ServiceLister
	secretLister  corev1.SecretLister
	hostAddr      string
	timeout       time.Duration
}

func NewProber(
	namespace string,
	serviceName string,
	secretName string,
	serviceLister corev1.ServiceLister,
	secretLister corev1.SecretLister,
	hostAddr string,
) *Prober {
	return &Prober{
		namespace:     namespace,
		serviceName:   serviceName,
		secretName:    secretName,
		serviceLister: serviceLister,
		secretLister:  secretLister,
		hostAddr:      hostAddr,
		timeout:       60 * time.Second,
	}
}

func (p *Prober) serviceRef() string {
	return fmt.Sprintf("%s/%s", p.namespace, p.serviceName)
}

func (p *Prober) getScyllaClient() (*scyllaclient.Client, error) {
	secret, err := p.secretLister.Secrets(p.namespace).Get(p.secretName)
	if err != nil {
		return nil, fmt.Errorf("can't get manager agent auth secret %s/%s: %w", p.namespace, p.secretName, err)
	}

	return controllerhelpers.NewScyllaClientFromSecret(secret, []string{p.hostAddr})
}

func (p *Prober) isNodeUnderMaintenance() (bool, error) {
	svc, err := p.serviceLister.Services(p.namespace).Get(p.serviceName)
	if err != nil {
		return false, err
	}

	_, hasLabel := svc.Labels[naming.NodeMaintenanceLabel]
	return hasLabel, nil
}

func (p *Prober) getNodeAddress() (string, error) {
	svc, err := p.serviceLister.Services(p.namespace).Get(p.serviceName)
	if err != nil {
		return "", err
	}

	return controllerhelpers.GetScyllaIPFromService(svc)
}

func (p *Prober) Readyz(w http.ResponseWriter, req *http.Request) {
	ctx, ctxCancel := context.WithTimeout(req.Context(), p.timeout)
	defer ctxCancel()

	underMaintenance, err := p.isNodeUnderMaintenance()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		klog.ErrorS(err, "readyz probe: can't look up service maintenance label", "Service", p.serviceRef())
		return
	}

	if underMaintenance {
		// During maintenance Pod shouldn't be declare to be ready.
		w.WriteHeader(http.StatusServiceUnavailable)
		klog.V(2).InfoS("readyz probe: node is under maintenance", "Service", p.serviceRef())
		return
	}

	scyllaClient, err := p.getScyllaClient()
	if err != nil {
		klog.ErrorS(err, "readyz probe: can't get scylla client", "Service", p.serviceRef())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Contact Scylla to learn about the status of the member
	nodeStatuses, err := scyllaClient.Status(ctx, p.hostAddr)
	if err != nil {
		klog.ErrorS(err, "readyz probe: can't get scylla node status", "Service", p.serviceRef())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	nodeAddress, err := p.getNodeAddress()
	if err != nil {
		klog.ErrorS(err, "readyz probe: can't get scylla node address", "Service", p.serviceRef())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, s := range nodeStatuses {
		klog.V(4).InfoS("readyz probe: node state", "Node", s.Addr, "Status", s.Status, "State", s.State)

		if s.Addr == nodeAddress && s.IsUN() {
			transportEnabled, err := scyllaClient.IsNativeTransportEnabled(ctx, p.hostAddr)
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				klog.ErrorS(err, "readyz probe: can't get scylla native transport", "Service", p.serviceRef(), "Node", s.Addr)
				return
			}

			klog.V(4).InfoS("readyz probe: node state", "Node", s.Addr, "NativeTransportEnabled", transportEnabled)
			if transportEnabled {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
	}

	klog.V(2).InfoS("readyz probe: node is not ready", "Service", p.serviceRef())
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (p *Prober) Healthz(w http.ResponseWriter, req *http.Request) {
	underMaintenance, err := p.isNodeUnderMaintenance()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		klog.ErrorS(err, "healthz probe: can't look up service maintenance label", "Service", p.serviceRef())
		return
	}

	if underMaintenance {
		w.WriteHeader(http.StatusOK)
		klog.V(2).InfoS("healthz probe: node is under maintenance", "Service", p.serviceRef())
		return
	}

	host, err := network.FindFirstNonLocalIP()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		klog.ErrorS(err, "healthz probe: can't determine the local IP", "Service", p.serviceRef())
		return
	}

	scyllaClient, err := p.getScyllaClient()
	if err != nil {
		klog.ErrorS(err, "healthz probe: can't get scylla client", "Service", p.serviceRef())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Check if Scylla is reachable
	_, err = scyllaClient.Ping(context.Background(), host.String())
	if err != nil {
		klog.ErrorS(err, "healthz probe: can't connect to Scylla", "Service", p.serviceRef())
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}
