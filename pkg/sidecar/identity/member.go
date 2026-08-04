package identity

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
)

// seedResolutionPollInterval is how often an unready fallback seed candidate's broadcast address is
// re-checked while waiting for it to be assigned. It's a fixed, short interval because resolving a
// broadcast address is quick and there's no benefit in waiting longer between attempts.
const seedResolutionPollInterval = 500 * time.Millisecond

// seedResolutionPollTimeout bounds how long we wait for an unready fallback seed candidate's broadcast address
// to become resolvable. In practice the container's startup and liveness probes should fail and restart the
// process long before this fires, so the timeout is only a defensive backstop against waiting forever.
const seedResolutionPollTimeout = 15 * time.Minute

// Member encapsulates the identity for a single member
// of a Scylla Cluster.
type Member struct {
	// Name of the Pod
	Name string
	// Namespace of the Pod
	Namespace     string
	Rack          string
	Datacenter    string
	Cluster       string
	ServiceLabels map[string]string
	PodID         string

	Overprovisioned             bool
	BroadcastRPCAddress         string
	BroadcastAddress            string
	AdditionalScyllaDBArguments []string

	NodesBroadcastAddressType scyllav1alpha1.BroadcastAddressType
	IPFamily                  corev1.IPFamily
}

func NewMember(service *corev1.Service, pod *corev1.Pod, nodesAddressType, clientAddressType scyllav1alpha1.BroadcastAddressType, ipFamily corev1.IPFamily, additionalScyllaDBArguments []string) (*Member, error) {
	var err error

	m := &Member{
		Namespace:                   service.Namespace,
		Name:                        service.Name,
		Rack:                        pod.Labels[naming.RackNameLabel],
		Datacenter:                  pod.Labels[naming.DatacenterNameLabel],
		Cluster:                     pod.Labels[naming.ClusterNameLabel],
		ServiceLabels:               service.Labels,
		PodID:                       string(pod.UID),
		Overprovisioned:             pod.Status.QOSClass != corev1.PodQOSGuaranteed,
		NodesBroadcastAddressType:   nodesAddressType,
		IPFamily:                    ipFamily,
		AdditionalScyllaDBArguments: additionalScyllaDBArguments,
	}

	m.BroadcastAddress, err = controllerhelpers.GetScyllaBroadcastAddress(nodesAddressType, service, pod, &ipFamily)
	if err != nil {
		return nil, fmt.Errorf("can't get node broadcast address: %w", err)
	}

	m.BroadcastRPCAddress, err = controllerhelpers.GetScyllaBroadcastAddress(clientAddressType, service, pod, &ipFamily)
	if err != nil {
		return nil, fmt.Errorf("can't get client broadcast address: %w", err)
	}

	return m, nil
}

// seedCandidate is a node in this member's DC, which may end up being selected as its seed.
type seedCandidate struct {
	pod *corev1.Pod
	svc *corev1.Service
}

// GetSeeds returns the seeds for this member. External seeds, if provided, are always included.
// If no ready seed candidate exists, it blocks polling until the selected fallback candidate's
// broadcast address becomes resolvable or ctx is canceled.
func (m *Member) GetSeeds(ctx context.Context, coreClient corev1client.CoreV1Interface, externalSeeds []string) ([]string, error) {
	candidates, err := m.getDCSeedCandidates(ctx, coreClient)
	if err != nil {
		return nil, fmt.Errorf("can't get seed candidates: %w", err)
	}
	klog.V(4).InfoS("Found DC seed candidates", "Candidates", seedCandidateNames(candidates))

	// Self takes part in selection like any other node.
	selected, fallback := m.selectDCSeedCandidates(candidates)
	klog.V(4).InfoS("Selected DC seed candidates", "Candidates", seedCandidateNames(selected), "Fallback", fallback)

	if fallback {
		// No ready Pods exist - fall back to distinguished unready candidate.
		klog.V(2).InfoS("Falling back to distinguished DC seed candidate", "Candidate", selected[0].pod.Name, "ExternalSeeds", externalSeeds)
		return m.fallBackWithDistinguishedCandidate(ctx, coreClient, selected[0], externalSeeds)
	}

	klog.V(2).InfoS("Seeding off the selected DC seed candidates", "Candidates", seedCandidateNames(selected), "ExternalSeeds", externalSeeds)
	seeds := make([]string, 0, len(externalSeeds)+len(selected))
	seeds = append(seeds, externalSeeds...)

	var errs []error
	for _, s := range selected {
		resolved, err := m.resolveSeedBroadcastAddress(s)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't resolve seed broadcast address: %w", err))
			continue
		}
		seeds = append(seeds, resolved)
	}

	// Selected seed candidates must be ready so we assume their broadcast addresses should be resolvable and error otherwise.
	if len(errs) > 0 {
		return nil, fmt.Errorf("can't resolve seed broadcast addresses: %w", apimachineryutilerrors.NewAggregate(errs))
	}

	return seeds, nil
}

// fallBackWithDistinguishedCandidate returns the seeds for the case where no candidate was ready and a single, possibly not ready,
// candidate was distinguished. It blocks polling until the candidate's broadcast address becomes resolvable or ctx is canceled.
func (m *Member) fallBackWithDistinguishedCandidate(ctx context.Context, coreClient corev1client.CoreV1Interface, fallbackCandidate seedCandidate, externalSeeds []string) ([]string, error) {
	if fallbackCandidate.pod.Name == m.Name {
		// Avoid seeding of itself when external seeds are provided not to form a separate cluster.
		if len(externalSeeds) > 0 {
			klog.V(2).InfoS("Self is the fallback DC seed candidate, seeding off the external seeds only to avoid forming a separate cluster", "ExternalSeeds", externalSeeds)
			return externalSeeds, nil
		}

		// Self is the fallback node, and it isn't joining an existing cluster - seed of itself.
		klog.V(2).InfoS("Self is the fallback DC seed candidate and there are no external seeds, seeding off itself", "BroadcastAddress", m.BroadcastAddress)
		return []string{m.BroadcastAddress}, nil
	}

	seeds := make([]string, 0, len(externalSeeds)+1)
	seeds = append(seeds, externalSeeds...)

	// The fallback candidate isn't self and isn't ready, so its broadcast address may be unresolvable.
	// Wait for it rather than fail.
	klog.V(2).InfoS("Waiting for the DC fallback seed candidate's broadcast address to become resolvable", "Candidate", fallbackCandidate.pod.Name)
	var resolved string
	err := apimachineryutilwait.PollUntilContextTimeout(ctx, seedResolutionPollInterval, seedResolutionPollTimeout, true, func(ctx context.Context) (bool, error) {
		pod, err := coreClient.Pods(m.Namespace).Get(ctx, fallbackCandidate.pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("can't get pod %q: %w", fallbackCandidate.pod.Name, err)
		}
		svc, err := coreClient.Services(m.Namespace).Get(ctx, fallbackCandidate.svc.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("can't get service %q: %w", fallbackCandidate.svc.Name, err)
		}

		resolved, err = m.resolveSeedBroadcastAddress(seedCandidate{pod: pod, svc: svc})
		if err != nil {
			klog.V(4).InfoS("Fallback seed broadcast address not resolvable yet, retrying", "Candidate", fallbackCandidate.pod.Name, "Error", err)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("can't resolve fallback seed broadcast address of %q: %w", fallbackCandidate.pod.Name, err)
	}

	seeds = append(seeds, resolved)
	return seeds, nil
}

// getDCSeedCandidates returns all seed candidates, including itself, from this member's DC, in no particular order.
func (m *Member) getDCSeedCandidates(ctx context.Context, coreClient corev1client.CoreV1Interface) ([]seedCandidate, error) {
	clusterLabels := naming.ScyllaLabels()
	clusterLabels[naming.ClusterNameLabel] = m.Cluster

	// The Pod type narrows the selector to ScyllaDB nodes, excluding other Pods that could otherwise match cluster labels.
	nodePodLabels := maps.Clone(clusterLabels)
	nodePodLabels[naming.PodTypeLabel] = string(naming.PodTypeScyllaDBNode)

	podList, err := coreClient.Pods(m.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(nodePodLabels).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("can't list pods: %w", err)
	}

	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("internal error: can't find any pod for this cluster, including itself")
	}

	// The Service type narrows the selector to ScyllaDB nodes, excluding other Services that could otherwise match cluster labels.
	memberServiceLabels := maps.Clone(clusterLabels)
	memberServiceLabels[naming.ScyllaServiceTypeLabel] = string(naming.ScyllaServiceTypeMember)

	svcList, err := coreClient.Services(m.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(memberServiceLabels).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("can't list services: %w", err)
	}

	svcs := make(map[string]*corev1.Service, len(svcList.Items))
	for i := range svcList.Items {
		svc := &svcList.Items[i]
		svcs[svc.Name] = svc
	}

	var errs []error
	candidates := make([]seedCandidate, 0, len(podList.Items))
	for i := range podList.Items {
		pod := &podList.Items[i]

		if len(pod.Labels[naming.RackNameLabel]) == 0 {
			errs = append(errs, fmt.Errorf("pod %q is missing %q label", naming.ObjRef(pod), naming.RackNameLabel))
			continue
		}

		// Members share their name with their Service.
		svc, ok := svcs[pod.Name]
		if !ok {
			errs = append(errs, fmt.Errorf("can't find service for pod %q", naming.ObjRef(pod)))
			continue
		}

		candidates = append(candidates, seedCandidate{pod: pod, svc: svc})
	}

	if err := apimachineryutilerrors.NewAggregate(errs); err != nil {
		return nil, err
	}

	return candidates, nil
}

// seedCandidateNames returns the Pod/Service names of the given candidates.
func seedCandidateNames(candidates []seedCandidate) []string {
	return slices.Collect(func(yield func(string) bool) {
		for _, c := range candidates {
			if !yield(c.pod.Name) {
				return
			}
		}
	})
}

// selectDCSeedCandidates returns the candidates to seed off, given all candidates from a DC in no particular order,
// and whether it had to fall back to a single, possibly not ready, candidate because none was ready.
func (m *Member) selectDCSeedCandidates(candidates []seedCandidate) ([]seedCandidate, bool) {
	// Rank the candidates by the creation timestamp of their Service, with the name as a tiebreak.
	// Ranking deliberately depends on nothing but those two, because every node selects its own seeds
	// independently, and they have to agree: two nodes each picking the other as its seed would never form a
	// cluster. Service creation time serves as the ordering input that outlives the Pods.
	rankedCandidates := slices.Clone(candidates)
	slices.SortFunc(rankedCandidates, func(a, b seedCandidate) int {
		return cmp.Or(
			a.svc.CreationTimestamp.Time.Compare(b.svc.CreationTimestamp.Time),
			// Members created within the same second tie. Fall back to the name to keep the order total.
			strings.Compare(a.pod.Name, b.pod.Name),
		)
	})

	// If any candidate is ready, the first ready one of each rack is selected, since a seed which is already
	// serving lets the node join without waiting. The readiness probe only passes once ScyllaDB reports the
	// node as UN with the native transport enabled, so it's a liveness signal, not just a Kubernetes one.
	var selected []seedCandidate
	seenRacks := map[string]bool{}
	for _, c := range rankedCandidates {
		// Self can't be ready as seed selection runs before ScyllaDB starts,
		// but it can appear so after a restart before the condition is flipped.
		// Self is still considered in a fallback path.
		if c.pod.Name == m.Name {
			continue
		}

		rack := c.pod.Labels[naming.RackNameLabel]
		if !controllerhelpers.IsPodReady(c.pod) || seenRacks[rack] {
			continue
		}
		seenRacks[rack] = true

		selected = append(selected, c)
	}

	// Otherwise, the first-ranked candidate is selected on its own - it's the one all nodes agree on.
	// Selecting a seed which isn't serving yet is safe: a restarting node ignores the seeds, contacting the
	// peers persisted in system.peers instead, and a bootstrapping node keeps retrying group 0 discovery until a seed responds.
	// Node replacement requires a live seed, but it replaces a node of an otherwise running cluster,
	// so a ready candidate is expected to exist.
	if len(selected) == 0 {
		return rankedCandidates[:1], true
	}

	return selected, false
}

// resolveSeedBroadcastAddress returns the broadcast address of the given candidate.
func (m *Member) resolveSeedBroadcastAddress(candidate seedCandidate) (string, error) {
	if candidate.pod.Name == m.Name {
		// This node's own address has already been resolved when the member was built.
		return m.BroadcastAddress, nil
	}

	// Assume nodes share broadcast address type and IP family and they are immutable.
	broadcastAddress, err := controllerhelpers.GetScyllaBroadcastAddress(m.NodesBroadcastAddressType, candidate.svc, candidate.pod, &m.IPFamily)
	if err != nil {
		return "", fmt.Errorf("can't get broadcast address of %q: %w", candidate.pod.Name, err)
	}

	return broadcastAddress, nil
}
