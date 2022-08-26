package sidecar

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

const (
	localhost = "localhost"
)

type Prober struct {
	namespace     string
	serviceName   string
	serviceLister corev1.ServiceLister
	timeout       time.Duration
}

func NewProber(
	namespace string,
	serviceName string,
	serviceLister corev1.ServiceLister,
) *Prober {
	return &Prober{
		namespace:     namespace,
		serviceName:   serviceName,
		serviceLister: serviceLister,
		timeout:       60 * time.Second,
	}
}

func (p *Prober) serviceRef() string {
	return fmt.Sprintf("%s/%s", p.namespace, p.serviceName)
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

	scyllaClient, err := controllerhelpers.NewScyllaClientForLocalhost()
	if err != nil {
		klog.ErrorS(err, "readyz probe: can't get scylla client", "Service", p.serviceRef())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer scyllaClient.Close()

	// Contact Scylla to learn about the status of the member
	nodeStatuses, err := scyllaClient.Status(ctx, localhost)
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
			transportEnabled, err := scyllaClient.IsNativeTransportEnabled(ctx, localhost)
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
	ctx, ctxCancel := context.WithTimeout(req.Context(), p.timeout)
	defer ctxCancel()

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

	scyllaClient, err := controllerhelpers.NewScyllaClientForLocalhost()
	if err != nil {
		klog.ErrorS(err, "healthz probe: can't get scylla client", "Service", p.serviceRef())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer scyllaClient.Close()

	// Check if Scylla API is reachable
	_, err = scyllaClient.Ping(ctx, localhost)
	if err != nil {
		klog.ErrorS(err, "healthz probe: can't connect to Scylla API", "Service", p.serviceRef())
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}
