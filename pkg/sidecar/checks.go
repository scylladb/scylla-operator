package sidecar

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/naming"
	log "github.com/sirupsen/logrus"
	"github.com/yanniszark/go-nodetool/nodetool"
	"net/http"
)

// setupHTTPChecks brings up the liveness and readiness probes.
// Blocks. Meant to be run as a goroutine.
func (mc *MemberController) setupHTTPChecks() {

	http.HandleFunc(naming.LivenessProbePath, livenessCheck(mc))
	http.HandleFunc(naming.ReadinessProbePath, readinessCheck(mc))

	err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", naming.ProbePort), nil)
	// If ListenAndServe returns, something went wrong
	log.Fatalf("Error in HTTP checks: %+v", errors.WithStack(err))
}

func livenessCheck(mc *MemberController) func(http.ResponseWriter, *http.Request) {

	return func(w http.ResponseWriter, req *http.Request) {

		status := http.StatusOK

		// Check if JMX is reachable
		_, err := mc.nodetool.Status()
		if err != nil {
			log.Errorf("Liveness check failed with error: %v", err)
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
			log.Errorf("Readiness check failed with error: %v", err)
			status = http.StatusServiceUnavailable
		}
		w.WriteHeader(status)
	}
}
