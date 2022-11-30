package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
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

func GetInternalNodeStatuses(ctx context.Context, addr string) (scyllaclient.NodeStatusInfoSlice, error) {
	statuses := scyllaclient.NodeStatusInfoSlice{}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return statuses, fmt.Errorf("can't create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		return statuses, fmt.Errorf("can't send a request: %w", err)
	}
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&statuses)
	if err != nil {
		return statuses, fmt.Errorf("can't decode json: %w", err)
	}

	return statuses, nil
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

	klog.V(4).InfoS("readyz probe: node statuses", "Statuses", nodeStatuses)

	// Check for self UN and isNativeTransportEnabled first.
	selfStatusFound := false
	for _, s := range nodeStatuses {
		if s.Addr != nodeAddress {
			continue
		}

		selfStatusFound = true

		if !s.IsUN() {
			w.WriteHeader(http.StatusServiceUnavailable)
			klog.V(2).InfoS("readyz probe: node is not UN", "Service", p.serviceRef(), "Node", s.Addr)
			return
		}

		transportEnabled, err := scyllaClient.IsNativeTransportEnabled(ctx, localhost)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			klog.ErrorS(err, "readyz probe: can't get scylla native transport", "Service", p.serviceRef(), "Node", s.Addr)
			return
		}

		if !transportEnabled {
			w.WriteHeader(http.StatusServiceUnavailable)
			klog.V(2).InfoS("readyz probe: scylla native transport is not enabled", "Service", p.serviceRef(), "Node", s.Addr)
			return
		}
	}

	if !selfStatusFound {
		w.WriteHeader(http.StatusServiceUnavailable)
		klog.V(2).InfoS("readyz probe: node's own status is missing", "Service", p.serviceRef())
		return
	}

	receivedStatuses := make(map[string]scyllaclient.NodeStatusInfoSlice)
	receivedStatuses[nodeAddress] = nodeStatuses

	notUNMap := make(map[string]bool)
	for _, s := range nodeStatuses {
		if !s.IsUN() {
			notUNMap[s.Addr] = true
		}
	}

	if len(notUNMap) > 1 {
		w.WriteHeader(http.StatusServiceUnavailable)
		klog.V(2).InfoS("readyz probe: node considers more than one peer not UN", "Service", p.serviceRef(), "Statuses", nodeStatuses)
		return
	}

	errs := make([]error, 0)
	for _, s := range nodeStatuses {
		if s.Addr == nodeAddress {
			continue
		}

		if notUNMap[s.Addr] {
			continue
		}

		statusURL := url.URL{
			Scheme: "http",
			Host:   net.JoinHostPort(s.Addr, strconv.Itoa(naming.ProbePort)),
			Path:   naming.InternalNodeStatusesPath,
		}
		statuses, err := GetInternalNodeStatuses(ctx, statusURL.String())
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get statuses from node %s: %w", s.Addr, err))
			break
		}

		klog.V(4).InfoS("readyz probe: received statuses", "Node", s.Addr, "Statuses", statuses)
		receivedStatuses[s.Addr] = statuses

		for _, rs := range statuses {
			if !rs.IsUN() {
				notUNMap[rs.Addr] = true
			}
		}
	}

	err = apierrors.NewAggregate(errs)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		klog.ErrorS(err, "readyz probe: can't gather statuses from peers", "Service", p.serviceRef(), "Err", err)
		return
	}

	if len(notUNMap) > 1 {
		w.WriteHeader(http.StatusServiceUnavailable)
		klog.V(2).InfoS("readyz probe: more than one node considered not UN by peers", "Service", p.serviceRef(), "Statuses", receivedStatuses)
		return
	}

	klog.V(2).InfoS("readyz probe: node is ready", "Service", p.serviceRef())
	w.WriteHeader(http.StatusOK)
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

func (p *Prober) InternalNodeStatuses(w http.ResponseWriter, req *http.Request) {
	ctx, ctxCancel := context.WithTimeout(req.Context(), p.timeout)
	defer ctxCancel()

	scyllaClient, err := controllerhelpers.NewScyllaClientForLocalhost()
	if err != nil {
		klog.ErrorS(err, "internal_node_statuses: can't get scylla client", "Service", p.serviceRef())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer scyllaClient.Close()

	nodeStatuses, err := scyllaClient.Status(ctx, localhost)
	if err != nil {
		klog.ErrorS(err, "internal_node_statuses: can't get scylla node status", "Service", p.serviceRef())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	payload, err := json.Marshal(nodeStatuses)
	if err != nil {
		klog.ErrorS(err, "internal_node_statuses: can't marshall node statuses", "Service", p.serviceRef())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)
}
