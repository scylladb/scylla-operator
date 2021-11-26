// Copyright (C) 2021 ScyllaDB

package cloud

import (
	"net"
	"net/http"

	"cloud.google.com/go/compute/metadata"
)

type gkeInstanceMetadata struct {
	InstanceName string
}

func discoverGKE(client *http.Client) (*gkeInstanceMetadata, error) {
	c := metadata.NewClient(client)
	in, err := c.InstanceName()
	if err != nil {
		return nil, err
	}

	im := &gkeInstanceMetadata{
		InstanceName: in,
	}

	return im, nil
}

func OnGKE() bool {
	addrs, err := net.LookupHost("metadata.google.internal")
	if err != nil || len(addrs) == 0 {
		return false
	}
	return true
}
