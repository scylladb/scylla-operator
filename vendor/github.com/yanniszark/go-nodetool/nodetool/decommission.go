package nodetool

import "github.com/yanniszark/go-nodetool/client"

func (t *Nodetool) Decommission() error {

	req := &client.JolokiaExecRequest{
		MBean:     storageServiceMBean,
		Operation: "decommission",
	}
	// If operation succeeded we don't need to know anything else
	_, err := t.Client.Do(req)
	return err
}
