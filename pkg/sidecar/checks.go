package sidecar

import (
	"context"
	"fmt"
	"net/http"

	"github.com/scylladb/scylla-operator/pkg/util/network"

	"github.com/pkg/errors"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/naming"
)

// setupHTTPChecks brings up the liveness and readiness probes.
// Blocks. Meant to be run as a goroutine.
func (mc *MemberController) setupHTTPChecks(ctx context.Context) {

	http.HandleFunc(naming.LivenessProbePath, livenessCheck(mc))
	http.HandleFunc(naming.ReadinessProbePath, readinessCheck(mc))

	if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", naming.ProbePort), nil); err != nil {
		mc.logger.Fatal(ctx, "Error in HTTP checks", "error", errors.WithStack(err))
	}
}

func livenessCheck(mc *MemberController) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		host, err := network.FindFirstNonLocalIP()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			mc.logger.Error(log.WithTraceID(req.Context()), "Liveness check failed", "error", err)
			return
		}
		// Check if JMX is reachable
		_, err = mc.scyllaClient.Status(context.Background(), host.String())
		if err != nil {
			mc.logger.Error(log.WithTraceID(req.Context()), "Liveness check failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func readinessCheck(mc *MemberController) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		host, err := network.FindFirstNonLocalIP()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			mc.logger.Error(log.WithTraceID(req.Context()), "Readiness check failed", "error", err)
			return
		}
		// Contact Scylla to learn about the status of the member
		hosts, err := mc.scyllaClient.Status(context.Background(), host.String())
		if err != nil {
			mc.logger.Error(log.WithTraceID(req.Context()), "error while executing nodetool status in readiness check", "error", err)
		}
		for _, h := range hosts {
			if h.Addr == mc.member.StaticIP && h.IsUN() {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		mc.logger.Error(log.WithTraceID(req.Context()), "Readiness check failed, node not ready")
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}
