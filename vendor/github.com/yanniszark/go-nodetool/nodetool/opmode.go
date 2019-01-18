package nodetool

import (
	"encoding/json"
	"fmt"
	"github.com/yanniszark/go-nodetool/client"
	"k8s.io/apimachinery/pkg/util/sets"
)

type NodeOperationMode string

const (
	NodeOperationModeStarting       = "STARTING"
	NodeOperationModeNormal         = "NORMAL"
	NodeOperationModeJoining        = "JOINING"
	NodeOperationModeLeaving        = "LEAVING"
	NodeOperationModeDecommissioned = "DECOMMISSIONED"
	NodeOperationModeMoving         = "MOVING"
	NodeOperationModeDraining       = "DRAINING"
	NodeOperationModeDrained        = "DRAINED"
)

type operationModeMBeanFields struct {
	OperationMode string `json:""`
}

// OperationMode gets the Operation Mode for the specific Cassandra instance.
// This command is available even after the node has decommissioned.
func (t *Nodetool) OperationMode() (NodeOperationMode, error) {

	req := &client.JolokiaReadRequest{
		MBean: storageServiceMBean,
		Attribute: []string{
			"OperationMode",
		},
	}
	res, err := t.Client.Do(req)
	if err != nil {
		return "", err
	}
	ssInfo := &operationModeMBeanFields{}
	if err := json.Unmarshal(res, ssInfo); err != nil {
		return "", err
	}

	opMode := ssInfo.OperationMode
	knownModes := sets.NewString(
		NodeOperationModeStarting,
		NodeOperationModeNormal,
		NodeOperationModeJoining,
		NodeOperationModeLeaving,
		NodeOperationModeDecommissioned,
		NodeOperationModeMoving,
		NodeOperationModeDraining,
		NodeOperationModeDrained,
	)
	if !knownModes.Has(opMode) {
		return "", fmt.Errorf("unknown operation mode %s", opMode)
	}
	return NodeOperationMode(opMode), nil
}
