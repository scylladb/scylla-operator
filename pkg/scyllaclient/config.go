package scyllaclient

import (
	"fmt"
	"net/http"
	"time"

	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// Config specifies the Client configuration.
type Config struct {
	// Hosts specifies all the cluster hosts that for a pool of hosts for the
	// client.
	Hosts []string
	// Port specifies the default Scylla Manager agent port.
	Port string
	// Transport scheme HTTP or HTTPS.
	Scheme string
	// AuthToken specifies the authentication token.
	AuthToken string `yaml:"auth_token"`
	// Timeout specifies time to complete a single request to Scylla REST API
	// possibly including opening a TCP connection.
	Timeout time.Duration `yaml:"timeout"`
	// PoolDecayDuration specifies size of time window to measure average
	// request time in Epsilon-Greedy host pool.
	PoolDecayDuration time.Duration
	// Transport allows for setting a custom round tripper to send HTTP
	// requests over not standard connections i.e. over SSH tunnel.
	Transport http.RoundTripper
}

// DefaultConfig returns a Config initialized with default values.
func DefaultConfig(authToken string, hosts ...string) *Config {
	return &Config{
		AuthToken: authToken,
		Hosts:     hosts,
		Port:      "10001",
		Scheme:    "https",
		Timeout:   15 * time.Second,

		PoolDecayDuration: 30 * time.Minute,
	}
}

// Validate checks if all the fields are properly set.
func (c *Config) Validate() error {
	var errs []error

	if len(c.Hosts) == 0 {
		errs = append(errs, fmt.Errorf("missing hosts"))
	}
	if c.Port == "" {
		errs = append(errs, fmt.Errorf("missing port"))
	}

	return apimachineryutilerrors.NewAggregate(errs)
}
