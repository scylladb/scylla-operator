package nodetool

import (
	"encoding/json"
	"github.com/yanniszark/go-nodetool/client"
	"github.com/yanniszark/go-nodetool/version"
)

type versionMBeanFields struct {
	ReleaseVersion *version.Version `json:""`
}

func (t *Nodetool) Version() (*version.Version, error) {

	req := &client.JolokiaReadRequest{
		MBean: storageServiceMBean,
		Attribute: []string{
			"ReleaseVersion",
		},
	}
	res, err := t.Client.Do(req)
	if err != nil {
		return nil, err
	}

	ssInfo := &versionMBeanFields{}
	if err := json.Unmarshal(res, ssInfo); err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	return ssInfo.ReleaseVersion, nil
}
