package gapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// This function creates a API key inside the Grafana instance running in stack `stack`. It's used in order
// to provision API keys inside Grafana while just having access to a Grafana Cloud API key.
//
// See https://grafana.com/docs/grafana-cloud/api/#create-grafana-api-keys for more information.
func (c *Client) CreateGrafanaAPIKeyFromCloud(stack string, input *CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	resp := &CreateAPIKeyResponse{}
	err = c.request("POST", fmt.Sprintf("/api/instances/%s/api/auth/keys", stack), nil, bytes.NewBuffer(data), resp)
	return resp, err
}

// The Grafana Cloud API is disconnected from the Grafana API on the stacks unfortunately. That's why we can't use
// the Grafana Cloud API key to fully manage API keys on the Grafana API. The only thing we can do is to create
// a temporary Admin key, and create a Grafana API client with that.
func (c *Client) CreateTemporaryStackGrafanaClient(stackSlug, tempKeyPrefix string, tempKeyDuration time.Duration) (tempClient *Client, cleanup func() error, err error) {
	stack, err := c.StackBySlug(stackSlug)
	if err != nil {
		return nil, nil, err
	}

	name := fmt.Sprintf("%s-%d", tempKeyPrefix, time.Now().UnixNano())
	req := &CreateAPIKeyRequest{
		Name:          name,
		Role:          "Admin",
		SecondsToLive: int64(tempKeyDuration.Seconds()),
	}

	apiKey, err := c.CreateGrafanaAPIKeyFromCloud(stackSlug, req)
	if err != nil {
		return nil, nil, err
	}

	client, err := New(stack.URL, Config{APIKey: apiKey.Key})
	if err != nil {
		return nil, nil, err
	}

	cleanup = func() error {
		_, err = client.DeleteAPIKey(apiKey.ID)
		return err
	}

	return client, cleanup, nil
}
