package transport

import (
	"log"
	"sort"
)

// HostSelectionPolicy decides which node the query should be routed to.
type HostSelectionPolicy interface {
	// Returns i-th node of the host selection plan, returns nil after going through the whole plan.
	Node(QueryInfo, int) *Node
}

type TokenAwarePolicy struct {
	localDC string
}

func NewTokenAwarePolicy(localDC string) *TokenAwarePolicy {
	return &TokenAwarePolicy{localDC: localDC}
}

func (p *TokenAwarePolicy) Node(qi QueryInfo, offset int) *Node {
	if p.localDC == "" {
		var replicas []*Node
		pi := qi.topology.policyInfo
		if qi.tokenAware {
			pos := pi.ring.tokenLowerBound(qi.token)
			replicas = pi.ring[pos].localReplicas
		} else {
			// Fallback to round robin on all nodes.
			replicas = pi.localNodes
		}

		if offset >= len(replicas) {
			return nil
		}

		idx := (qi.offset + uint64(offset)) % uint64(len(replicas))
		return replicas[idx]
	}

	var local, remote []*Node
	pi := qi.topology.policyInfo
	if qi.tokenAware {
		pos := pi.ring.tokenLowerBound(qi.token)
		local = pi.ring[pos].localReplicas
		remote = pi.ring[pos].remoteReplicas
	} else {
		// Fallback to DC aware round robin on all nodes.
		local = pi.localNodes
		remote = pi.remoteNodes
	}

	if offset < len(local) {
		idx := (qi.offset + uint64(offset)) % uint64(len(local))
		return local[idx]
	} else if offset < len(local)+len(remote) {
		idx := (qi.offset + uint64(offset) - uint64(len(local))) % uint64(len(remote))
		return remote[idx]
	}

	return nil
}

type policyInfo struct {
	ring Ring

	localNodes  []*Node
	remoteNodes []*Node
}

func (pi *policyInfo) Preprocess(t *topology, ks keyspace) {
	switch ks.strategy.class {
	case simpleStrategy, localStrategy:
		pi.preprocessSimpleStrategy(t, ks.strategy)
	case networkTopologyStrategy:
		pi.preprocessNetworkTopologyStrategy(t, ks.strategy)
	default:
		log.Println("policyInfo: unknown strategy, defaulting to round robin")
		if t.localDC == "" {
			pi.preprocessRoundRobinStrategy(t)
		} else {
			pi.preprocessDCAwareRoundRobinStrategy(t)
		}
	}
}

func (pi *policyInfo) preprocessSimpleStrategy(t *topology, stg strategy) {
	pi.localNodes = t.Nodes
	sort.Sort(pi.ring)
	trie := trieRoot()
	for i := range pi.ring {
		rit := replicaIter{
			ring:    pi.ring,
			offset:  i,
			fetched: 0,
		}

		filter := func(n *Node, res []*Node) bool {
			for _, v := range res {
				if n.hostID == v.hostID {
					return false
				}
			}

			return true
		}

		cur := &trie
		for len(cur.Path()) < int(stg.rf) {
			n := rit.Next()
			if n == nil {
				break
			}

			if filter(n, cur.Path()) {
				cur = cur.Next(n)
			}
		}
		pi.ring[i].localReplicas = cur.Path()
	}
}

func (pi *policyInfo) preprocessRoundRobinStrategy(t *topology) {
	pi.localNodes = t.Nodes
	pi.remoteNodes = nil
}

func (pi *policyInfo) preprocessDCAwareRoundRobinStrategy(t *topology) {
	pi.localNodes = make([]*Node, 0)
	pi.remoteNodes = make([]*Node, 0)
	for _, v := range t.Nodes {
		if v.datacenter == t.localDC {
			pi.localNodes = append(pi.localNodes, v)
		} else {
			pi.remoteNodes = append(pi.remoteNodes, v)
		}
	}
}

func (pi *policyInfo) preprocessNetworkTopologyStrategy(t *topology, stg strategy) {
	sort.Sort(pi.ring)
	trie := trieRoot()
	for i := range pi.ring {
		rit := replicaIter{
			ring:    pi.ring,
			offset:  i,
			fetched: 0,
		}
		desiredCnt := 0
		// repeats store the amount of nodes from the same rack that we can take in given DC.
		repeats := make(map[string]int, len(stg.dcRF))
		for k, v := range stg.dcRF {
			desiredCnt += int(v)
			repeats[k] = int(v) - t.dcRacks[k]
		}

		filter := func(n *Node, res []*Node) bool {
			rf := stg.dcRF[n.datacenter]
			fromDC := 0
			fromRack := 0
			for _, v := range res {
				if n.addr == v.addr {
					return false
				}
				if n.datacenter == v.datacenter {
					fromDC++
					if n.rack == v.rack {
						fromRack++
					}
				}
			}

			if fromDC < int(rf) {
				if fromRack == 0 {
					return true
				}
				if repeats[n.datacenter] > 0 {
					repeats[n.datacenter]--
					return true
				}
			}
			return false
		}

		plan := make([]*Node, 0, desiredCnt)
		for len(plan) < desiredCnt {
			n := rit.Next()
			if n == nil {
				break
			}

			if filter(n, plan) {
				plan = append(plan, n)
			}
		}

		local := &trie
		remote := &trie
		for _, n := range plan {
			if n.datacenter == t.localDC {
				local = local.Next(n)
			} else {
				remote = remote.Next(n)
			}
		}

		pi.ring[i].localReplicas = local.Path()
		pi.ring[i].remoteReplicas = remote.Path()
	}
}
