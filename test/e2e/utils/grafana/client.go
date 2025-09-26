package grafana

import (
	"crypto/tls"
	"fmt"
	"net/url"

	"github.com/go-openapi/strfmt"
	grafanaclient "github.com/grafana/grafana-openapi-client-go/client"
	grafanasearch "github.com/grafana/grafana-openapi-client-go/client/search"
	"github.com/scylladb/scylla-operator/pkg/pointer"
)

type Client struct {
	c *grafanaclient.GrafanaHTTPAPI
}

type ClientOptions struct {
	URL      string
	Username string
	Password string
	TLS      *tls.Config
}

func NewClient(opts ClientOptions) (*Client, error) {
	u, err := url.Parse(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	cfg := grafanaclient.TransportConfig{
		Host:      u.Host,
		BasePath:  "/api",
		Schemes:   []string{u.Scheme},
		BasicAuth: url.UserPassword(opts.Username, opts.Password),
		TLSConfig: opts.TLS,
	}

	c := grafanaclient.NewHTTPClientWithConfig(strfmt.Default, &cfg)

	return &Client{
		c: c,
	}, nil
}

func (c *Client) Dashboards() ([]Dashboard, error) {
	const limit = 1000
	var (
		dashboards []Dashboard
		page       int64 = 1 // Grafana pages are 1-indexed.
	)

	for {
		searchParams := grafanasearch.NewSearchParams()
		searchParams.Limit = pointer.Ptr[int64](limit)
		searchParams.Type = pointer.Ptr("dash-db")
		searchParams.Page = pointer.Ptr(page)

		resp, err := c.c.Search.Search(searchParams)
		if err != nil {
			return nil, fmt.Errorf("failed to search dashboards (page %d): %w", page, err)
		}

		payload := resp.GetPayload()
		for _, hit := range payload {
			dashboards = append(dashboards, Dashboard{
				Title:       hit.Title,
				Type:        string(hit.Type),
				Tags:        hit.Tags,
				FolderTitle: hit.FolderTitle,
			})
		}

		if len(payload) < limit {
			break
		}
		page++
	}

	return dashboards, nil
}

func (c *Client) HomeDashboardUID() (string, error) {
	resp, err := c.c.Dashboards.GetHomeDashboard()
	if err != nil {
		return "", fmt.Errorf("failed to get home dashboard: %w", err)
	}

	if m, ok := resp.GetPayload().Dashboard.(map[string]interface{}); ok {
		if uid, ok := m["uid"].(string); ok {
			return uid, nil
		}
		return "", fmt.Errorf("home dashboard does not have a uid")
	}

	return "", fmt.Errorf("unexpected type for dashboard payload")
}
