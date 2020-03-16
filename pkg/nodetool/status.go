package nodetool

import "github.com/pborman/uuid"

type NodeState string

const (
	NodeStateNormal  NodeState = "Normal"
	NodeStateLeaving NodeState = "Leaving"
	NodeStateJoining NodeState = "Joining"
	NodeStateMoving  NodeState = "Moving"
)

type NodeStatus string

const (
	NodeStatusUnknown NodeStatus = "Unknown"
	NodeStatusUp      NodeStatus = "Up"
	NodeStatusDown    NodeStatus = "Down"
)

type Node struct {
	Host   string
	ID     uuid.UUID
	State  NodeState
	Status NodeStatus
}

type NodeMap map[string]*Node

func (t *Nodetool) Status() (NodeMap, error) {
	return NodeMap{}, nil
}
