package grafana

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"

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

// HomeDashboardTitle returns the title of the dashboard Grafana serves as its home dashboard.
// The title is used to identify the dashboard because Grafana no longer embeds the home dashboard
// in the response; it redirects to a synthetic UID ("default-home-dashboard") instead,
// hiding the UID of the configured dashboard file.
func (c *Client) HomeDashboardTitle() (string, error) {
	resp, err := c.c.Dashboards.GetHomeDashboard()
	if err != nil {
		return "", fmt.Errorf("failed to get home dashboard: %w", err)
	}

	payload := resp.GetPayload()

	if payload.RedirectURI == "" {
		return "", fmt.Errorf("home dashboard response does not have a redirect URI")
	}

	// The redirect URI has the form "/d/{uid}/{slug}".
	parts := strings.Split(strings.TrimPrefix(payload.RedirectURI, "/"), "/")
	if len(parts) < 2 || parts[0] != "d" || parts[1] == "" {
		return "", fmt.Errorf("unexpected home dashboard redirect URI %q", payload.RedirectURI)
	}
	uid := parts[1]

	dashboardResp, err := c.c.Dashboards.GetDashboardByUID(uid)
	if err != nil {
		return "", fmt.Errorf("failed to get home dashboard with UID %q: %w", uid, err)
	}

	if m, ok := dashboardResp.GetPayload().Dashboard.(map[string]interface{}); ok {
		if title, ok := m["title"].(string); ok {
			return title, nil
		}
		return "", fmt.Errorf("home dashboard with UID %q does not have a title", uid)
	}

	return "", fmt.Errorf("unexpected type for dashboard payload of home dashboard with UID %q", uid)
}

type DatasourceHealth struct {
	Message string
	OK      bool
}

func (c *Client) DatasourceHealth(datasourceName string) (DatasourceHealth, error) {
	resp, err := c.c.Datasources.GetDataSourceByName(datasourceName)
	if err != nil {
		return DatasourceHealth{}, fmt.Errorf("failed to get datasource %q: %w", datasourceName, err)
	}

	datasourceUID := resp.GetPayload().UID
	healthResp, err := c.c.Datasources.CheckDatasourceHealthWithUID(datasourceUID)
	if err != nil {
		return DatasourceHealth{}, fmt.Errorf("failed to check health for datasource %q (UID %s): %w", datasourceName, datasourceUID, err)
	}

	return DatasourceHealth{
		Message: healthResp.GetPayload().Message,
		OK:      healthResp.IsSuccess(),
	}, nil
}
