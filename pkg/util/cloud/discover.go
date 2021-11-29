// Copyright (C) 2021 ScyllaDB

package cloud

import (
	"context"
	"fmt"
	"net/http"
)

func Discover(ctx context.Context) (*InstanceMetadata, error) {
	client := http.DefaultClient

	im := &InstanceMetadata{
		CloudProvider: UnknownCloud,
	}

	if OnEKS() {
		eksIm, err := discoverEKS(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("get EKS instance metadata: %w", err)
		}
		im.CloudProvider = EKSCloud
		im.InstanceType = eksIm.InstanceType
	}

	if OnGKE() {
		gkeIm, err := discoverGKE(client)
		if err != nil {
			return nil, fmt.Errorf("get GKE instance metadata: %w", err)
		}
		im.CloudProvider = GKECloud
		//TODO: instange type
		im.InstanceType = gkeIm.InstanceName
	}

	return im, nil
}
