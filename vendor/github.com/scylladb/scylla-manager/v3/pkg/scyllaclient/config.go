// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"net/http"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

// Config specifies the Client configuration.
type Config struct {
	TimeoutConfig
	// Transport scheme HTTP or HTTPS.
	Scheme string `yaml:"scheme"`
	// Hosts specifies all the cluster hosts that for a pool of hosts for the
	// client.
	Hosts []string
	// Port specifies the default Scylla Manager agent port.
	Port string
	// AuthToken specifies the authentication token.
	AuthToken string
	// Transport allows for setting a custom round tripper to send HTTP
	// requests over not standard connections i.e. over SSH tunnel.
	Transport http.RoundTripper
}

// TimeoutConfig is the configuration of the connection exposed to users.
type TimeoutConfig struct {
	// Timeout specifies time to complete a single request to Scylla REST API
	// possibly including opening a TCP connection.
	// The timeout may be increased exponentially on retry after a timeout error.
	Timeout time.Duration `yaml:"timeout"`
	// MaxTimeout specifies the effective maximal timeout value after increasing Timeout on retry.
	MaxTimeout time.Duration `yaml:"max_timeout"`
	// ListTimeout specifies maximum time to complete an iterative remote
	// directory listing. The retrieval is performed in batches this timeout
	// applies to the time it take to retrieve a single batch.
	ListTimeout time.Duration `yaml:"list_timeout"`
	// Backoff specifies parameters of exponential backoff used when requests
	// from Scylla Manager to Scylla Agent fail.
	Backoff BackoffConfig `yaml:"backoff"`
	// InteractiveBackoff specifies backoff for interactive requests i.e.
	// originating from API / sctool.
	InteractiveBackoff BackoffConfig `yaml:"interactive_backoff"`
	// PoolDecayDuration specifies size of time window to measure average
	// request time in Epsilon-Greedy host pool.
	PoolDecayDuration time.Duration `yaml:"pool_decay_duration"`
}

// BackoffConfig specifies request exponential backoff parameters.
type BackoffConfig struct {
	WaitMin    time.Duration `yaml:"wait_min"`
	WaitMax    time.Duration `yaml:"wait_max"`
	MaxRetries uint64        `yaml:"max_retries"`
	Multiplier float64       `yaml:"multiplier"`
	Jitter     float64       `yaml:"jitter"`
}

func DefaultConfig() Config {
	return DefaultConfigWithTimeout(DefaultTimeoutConfig())
}

func DefaultConfigWithTimeout(c TimeoutConfig) Config {
	return Config{
		Scheme:        "https",
		Port:          "10001",
		TimeoutConfig: c,
	}
}

func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Timeout:     30 * time.Second,
		MaxTimeout:  1 * time.Hour,
		ListTimeout: 5 * time.Minute,
		Backoff: BackoffConfig{
			WaitMin:    1 * time.Second,
			WaitMax:    30 * time.Second,
			MaxRetries: 9,
			Multiplier: 2,
			Jitter:     0.2,
		},
		InteractiveBackoff: BackoffConfig{
			WaitMin:    time.Second,
			MaxRetries: 1,
		},
		PoolDecayDuration: 30 * time.Minute,
	}
}

// TestConfig is a convenience function equal to calling DefaultConfig and
// setting hosts and token manually.
func TestConfig(hosts []string, token string) Config {
	config := DefaultConfig()
	config.Hosts = hosts
	config.AuthToken = token

	config.Timeout = 5 * time.Second
	config.ListTimeout = 30 * time.Second
	config.Backoff.MaxRetries = 2
	config.Backoff.WaitMin = 200 * time.Millisecond

	return config
}

func (c Config) Validate() error {
	var err error
	if len(c.Hosts) == 0 {
		err = multierr.Append(err, errors.New("missing hosts"))
	}
	if c.Port == "" {
		err = multierr.Append(err, errors.New("missing port"))
	}

	return err
}
