package sidecar

import (
	"context"
	"fmt"
	"net/http"

	"github.com/scylladb/scylla-operator/pkg/controllers/sidecar/identity"
	"github.com/scylladb/scylla-operator/pkg/util/network"

	"github.com/pkg/errors"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/naming"
)

// setupHTTPChecks brings up the liveness and readiness probes.
// Blocks. Meant to be run as a goroutine.
func (mc *MemberReconciler) setupHTTPChecks(ctx context.Context) {

	http.HandleFunc(naming.LivenessProbePath, livenessCheck(mc))
	http.HandleFunc(naming.ReadinessProbePath, readinessCheck(mc))

	if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", naming.ProbePort), nil); err != nil {
		mc.logger.Fatal(ctx, "Error in HTTP checks", "error", errors.WithStack(err))
	}
}

func nodeUnderMaintenance(ctx context.Context, mc *MemberReconciler) (bool, error) {
	member, err := identity.Retrieve(ctx, mc.member.Name, mc.member.Namespace, mc.kubeClient)
	if err != nil {
		return false, errors.Wrap(err, "get member service")
	}

	_, ok := member.ServiceLabels[naming.NodeMaintenanceLabel]
	return ok, nil
}

func livenessCheck(mc *MemberReconciler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := log.WithTraceID(req.Context())

		if maintenance, err := nodeUnderMaintenance(ctx, mc); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			mc.logger.Error(ctx, "Liveness check failed", "error", err)
			return
		} else if maintenance {
			// During maintenance Pod should stay alive.
			w.WriteHeader(http.StatusOK)
			mc.logger.Info(ctx, "Node under maintenance")
			return
		}

		host, err := network.FindFirstNonLocalIP()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			mc.logger.Error(ctx, "Liveness check failed", "error", err)
			return
		}
		// Check if JMX is reachable
		_, err = mc.scyllaClient.Ping(context.Background(), host.String())
		if err != nil {
			mc.logger.Error(ctx, "Liveness check failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func readinessCheck(mc *MemberReconciler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := log.WithTraceID(req.Context())

		if maintenance, err := nodeUnderMaintenance(ctx, mc); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			mc.logger.Error(ctx, "Readiness check failed", "error", err)
			return
		} else if maintenance {
			// During maintenance Pod shouldn't be declare to be ready.
			w.WriteHeader(http.StatusServiceUnavailable)
			mc.logger.Info(ctx, "Node under maintenance")
			return
		}

		host, err := network.FindFirstNonLocalIP()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			mc.logger.Error(ctx, "Readiness check failed", "error", err)
			return
		}

		// Contact Scylla to learn about the status of the member
		hosts, err := mc.scyllaClient.Status(context.Background(), host.String())
		if err != nil {
			mc.logger.Error(ctx, "error while executing nodetool status in readiness check", "error", err)
		}

		for _, h := range hosts {
			mc.logger.Debug(ctx, "Host readiness", "host", h.Addr, "status", h.Status, "state", h.State)
		}
		for _, h := range hosts {
			if h.Addr == mc.member.StaticIP && h.IsUN() {
				transportEnabled, err := mc.scyllaClient.IsNativeTransportEnabled(ctx, host.String())
				if err != nil {
					w.WriteHeader(http.StatusServiceUnavailable)
					mc.logger.Error(ctx, "Readiness check failed", "error", err)
					return
				}

				mc.logger.Debug(ctx, "Host native transport", "host", h.Addr, "enabled", transportEnabled)
				if transportEnabled {
					w.WriteHeader(http.StatusOK)
					return
				}
			}
		}

		mc.logger.Error(ctx, "Readiness check failed, node not ready")
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}
