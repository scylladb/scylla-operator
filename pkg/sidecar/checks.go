package sidecar

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pkg/errors"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/yanniszark/go-nodetool/nodetool"
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
		status := http.StatusOK
		// Check if JMX is reachable
		_, err := mc.nodetool.Status()
		if err != nil {
			mc.logger.Error(log.WithTraceID(req.Context()), "Liveness check failed", "error", err)
			status = http.StatusServiceUnavailable
		}
		w.WriteHeader(status)
	}
}

func readinessCheck(mc *MemberController) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		status := http.StatusOK
		err := func() error {
			// Contact Scylla to learn about the status of the member
			HostIDMap, err := mc.nodetool.Status()
			if err != nil {
				return fmt.Errorf("Error while executing nodetool status in readiness check: %s", err.Error())
			}
			// Get local node through static ip
			localNode, ok := HostIDMap[mc.member.StaticIP]
			if !ok {
				return fmt.Errorf("Couldn't find node with ip %s in nodetool status.", mc.member.StaticIP)
			}
			// Check local node status
			// Up means the member is alive
			if localNode.Status != nodetool.NodeStatusUp {
				return fmt.Errorf("Unexpected local node status: %s", localNode.Status)
			}
			// Check local node state
			// Normal means that the member has completed bootstrap and joined the cluster
			if localNode.State != nodetool.NodeStateNormal {
				return fmt.Errorf("Unexpected local node state: %s", localNode.State)
			}
			return nil
		}()

		if err != nil {
			mc.logger.Error(log.WithTraceID(req.Context()), "Readiness check failed", "error", err)
			status = http.StatusServiceUnavailable
		}
		w.WriteHeader(status)
	}
}
