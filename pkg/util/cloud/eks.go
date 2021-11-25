// Copyright (C) 2021 ScyllaDB

package cloud

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

type eksInstanceMetadata struct {
	InstanceType string
}

type eksMetadataClient struct {
	client *imds.Client
}

const (
	eksInstanceTypePath        = "/instance-type"
	eksDefaultMetadataEndpoint = "http://169.254.169.254"
)

func newEksMetadataClient(ctx context.Context, client *http.Client) (*eksMetadataClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx, func(options *config.LoadOptions) error {
		options.HTTPClient = client
		return nil
	})
	if err != nil {
		return nil, err
	}

	mc := &eksMetadataClient{
		client: imds.NewFromConfig(cfg),
	}

	return mc, nil
}

func (c *eksMetadataClient) readString(ctx context.Context, path string) (string, error) {
	resp, err := c.client.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: path,
	})
	if err != nil {
		return "", fmt.Errorf("get metadata at path %q: %w", path, err)
	}

	buf, err := ioutil.ReadAll(resp.Content)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	return string(buf), nil
}

func (c *eksMetadataClient) GetInstanceMetadata(ctx context.Context) (*eksInstanceMetadata, error) {
	instanceType, err := c.readString(ctx, eksInstanceTypePath)
	if err != nil {
		return nil, fmt.Errorf("get instance type: %w", err)
	}

	im := &eksInstanceMetadata{
		InstanceType: instanceType,
	}

	return im, nil
}

func discoverEKS(ctx context.Context, client *http.Client) (*eksInstanceMetadata, error) {
	mc, err := newEksMetadataClient(ctx, client)
	if err != nil {
		return nil, err
	}

	return mc.GetInstanceMetadata(ctx)
}

func OnEKS() bool {
	resp, err := http.Get(eksDefaultMetadataEndpoint)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}
	return true
}
