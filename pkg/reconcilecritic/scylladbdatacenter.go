// Copyright (c) 2026 ScyllaDB.

package reconcilecritic

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/reconciletrace/critic"
)

// scyllaDBDatacenterConventions is what the critic has to know about the ScyllaDBDatacenter controller not to flag
// its design as a defect, and to tell a necessary disruption from an unnecessary one.
const scyllaDBDatacenterConventions = `
The controller manages a ScyllaDB datacenter: one StatefulSet per rack, one member Service per ScyllaDB node, and
their supporting objects.

- A reconcile runs a fixed sequence of sync steps (pkg/controller/scylladbdatacenter/sync.go): ServiceAccounts,
  RoleBindings, the ScyllaDB Manager agent token Secret, certificates (CAs, serving and client certificates in Secrets,
  CA bundles in ConfigMaps), the managed ScyllaDB config ConfigMap, the nodes status report, the rack StatefulSets,
  the member Services, the PodDisruptionBudget, Ingresses and Jobs. The status is written last.
- Each step reports its own <step>ControllerProgressing and <step>ControllerDegraded conditions; they are aggregated
  into the Available, Progressing and Degraded conditions of the status.
- Objects are applied with the scylla-operator.scylladb.com/managed-hash annotation, the hash of the required object.
  An update is sent only when that hash differs from the existing object's, so object updates carry real changes.
- The status is written only when it differs semantically from the current one.
- The rack StatefulSet's Pod template carries scylla-operator.scylladb.com/inputs-hash, the hash of the managed
  ScyllaDB config ConfigMap's data. A config change changes it and rolls the rack's Pods out. A rollout is necessary
  only when the Pod template or that config changes.
- Rack node counts change through the StatefulSet's scale subresource. On a scale-down, a node leaves the cluster
  before its Pod is removed: the controller requests the decommission on the node's member Service, the sidecar
  decommissions the node and labels the Service scylla/decommissioned, and only then the StatefulSet is scaled down.
  With parallel node operations disabled, nodes leave one at a time from the highest ordinal; with them enabled, the
  nodes of a scale-down leave at once.
`

// envtestConditions explains a trace recorded in envtest, where the test plays every other actor.
const envtestConditions = `
The trace was recorded in envtest: a real API server, but no kube-controller-manager, no kubelet and no ScyllaDB.
The test plays those actors. It marks StatefulSets as rolled out, sets the labels the sidecar would set on member
Services, and creates or deletes Pods. Those writes reach the controller as watch events of objects it didn't write;
they aren't the controller's actions. Waiting for them is expected.
`

// scyllaDBDatacenterCodeMap points the critic at the code of each part of the controller.
const scyllaDBDatacenterCodeMap = `
- pkg/controller/scylladbdatacenter/controller.go: the watches and how events map to ScyllaDBDatacenters.
- pkg/controller/scylladbdatacenter/sync.go: a reconcile, its sync steps and the status update.
- pkg/controller/scylladbdatacenter/sync_certs.go: CAs, certificates and the CQL connection config Secret.
- pkg/controller/scylladbdatacenter/sync_config.go: the managed ScyllaDB config ConfigMap.
- pkg/controller/scylladbdatacenter/sync_statefulsets.go: rack StatefulSets, scaling, decommission, upgrades.
- pkg/controller/scylladbdatacenter/sync_services.go: member Services and decommission requests.
- pkg/controller/scylladbdatacenter/status.go: the status computation and update.
- pkg/controller/scylladbdatacenter/resource.go: the required objects.
- pkg/resourceapply: applying objects with the managed hash.
`

// scyllaDBDatacenterRubric adds the domain's questions to the generic ones.
const scyllaDBDatacenterRubric = `
- Did any rack roll out without a change to its Pod template inputs?
- Did nodes leave the cluster in a different order or concurrency than the parallel node operations setting implies?
- Did a status condition stay Progressing after the work it reports was done, or flip back and forth?
- Did a sync step fail or wait on something an earlier step of the same reconcile had just created?
`

// ScyllaDBDatacenterContext returns the domain context of the ScyllaDBDatacenter controller. inEnvtest adds how to
// read a trace recorded in envtest.
func ScyllaDBDatacenterContext(inEnvtest bool) critic.ContextProvider {
	sections := []critic.Section{
		{Title: "Controller conventions (not defects)", Body: scyllaDBDatacenterConventions},
	}
	if inEnvtest {
		sections = append(sections, critic.Section{Title: "Where the trace comes from", Body: envtestConditions})
	}
	sections = append(sections,
		critic.Section{Title: "Code map (paths relative to the repository root)", Body: scyllaDBDatacenterCodeMap},
		critic.Section{Title: "Domain questions", Body: scyllaDBDatacenterRubric},
	)

	return critic.ContextProviderFunc(func(_ context.Context, e *critic.Episode) ([]critic.Section, error) {
		if !strings.Contains(e.Controller, "scylladbdatacenter") {
			return nil, nil
		}
		return sections, nil
	})
}
