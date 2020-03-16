package scyllaclient

import (
	"context"
	"sort"

	"github.com/scylladb/go-log"
	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/mermaid/scyllaclient"
	scyllaOperations "github.com/scylladb/scylla-operator/pkg/scyllaclient/internal/scylla/client/operations"
)

type Client struct {
	scyllaOps *scyllaOperations.Client
}

func createClient(logger log.Logger, host, token string) (*scyllaclient.Client, error) {
	config := scyllaclient.DefaultConfig()
	config.Hosts = []string{host}
	config.AuthToken = token

	return scyllaclient.NewClient(config, logger.Named("client"))
}

// HostDatacenter looks up the datacenter that the given host belongs to.
func (c *Client) HostDatacenter(ctx context.Context, host string) (dc string, err error) {
	resp, err := c.scyllaOps.SnitchDatacenterGet(&scyllaOperations.SnitchDatacenterGetParams{
		Context: ctx,
		Host:    &host,
	})
	if err != nil {
		return "", err
	}
	dc = resp.Payload

	return
}

func (c *Client) Status(ctx context.Context) (NodeStatusInfoSlice, error) {
	// Get all hosts
	resp, err := c.scyllaOps.StorageServiceHostIDGet(&scyllaOperations.StorageServiceHostIDGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}

	all := make([]NodeStatusInfo, len(resp.Payload))
	for i, p := range resp.Payload {
		all[i].Addr = p.Key
		all[i].HostID = p.Value
	}

	// Get host datacenter (hopefully cached)
	for i := range all {
		all[i].Datacenter, err = c.HostDatacenter(ctx, all[i].Addr)
		if err != nil {
			return nil, err
		}
	}

	// Get live nodes
	live, err := c.scyllaOps.GossiperEndpointLiveGet(&scyllaOperations.GossiperEndpointLiveGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeStatus(all, NodeStatusUp, live.Payload)

	// Get joining nodes
	joining, err := c.scyllaOps.StorageServiceNodesJoiningGet(&scyllaOperations.StorageServiceNodesJoiningGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeState(all, NodeStateJoining, joining.Payload)

	// Get leaving nodes
	leaving, err := c.scyllaOps.StorageServiceNodesLeavingGet(&scyllaOperations.StorageServiceNodesLeavingGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeState(all, NodeStateLeaving, leaving.Payload)

	// Get moving nodes
	moving, err := c.scyllaOps.StorageServiceNodesMovingGet(&scyllaOperations.StorageServiceNodesMovingGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeState(all, NodeStateMoving, moving.Payload)

	// Sort by Datacenter and Address
	sort.Slice(all, func(i, j int) bool {
		if all[i].Datacenter != all[j].Datacenter {
			return all[i].Datacenter < all[j].Datacenter
		}
		return all[i].Addr < all[j].Addr
	})

	return all, nil
}

func setNodeState(all []NodeStatusInfo, state NodeState, addrs []string) {
	if len(addrs) == 0 {
		return
	}
	m := strset.New(addrs...)

	for i := range all {
		if m.Has(all[i].Addr) {
			all[i].State = state
		}
	}
}

func setNodeStatus(all []NodeStatusInfo, status NodeStatus, addrs []string) {
	if len(addrs) == 0 {
		return
	}
	m := strset.New(addrs...)

	for i := range all {
		if m.Has(all[i].Addr) {
			all[i].Status = status
		}
	}
}
