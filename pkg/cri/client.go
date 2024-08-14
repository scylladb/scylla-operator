// Copyright (C) 2021 ScyllaDB

package cri

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"
)

const (
	dialTimeout = 2 * time.Second
)

var (
	backoffConfig = backoff.Config{
		BaseDelay:  100 * time.Millisecond,
		Multiplier: 1.6,
		Jitter:     0.2,
		// MaxDelay shall not be close or longer than the sync loop timeout!
		MaxDelay: 10 * time.Second,
	}
)

var (
	NotFoundErr = fmt.Errorf("not found")
)

type endpointState struct {
	endpoint   string
	connection *grpc.ClientConn
	err        error
}

func connectToEndpoint(ctx context.Context, endpoint string) *endpointState {
	connCtx, connCtxCancel := context.WithTimeout(ctx, dialTimeout)
	defer connCtxCancel()

	u, err := url.Parse(endpoint)
	if err != nil {
		return &endpointState{
			endpoint:   endpoint,
			connection: nil,
			err:        fmt.Errorf("invalid url: %w", err),
		}
	}

	var dialer func(context.Context, string) (net.Conn, error)
	switch u.Scheme {
	case "unix":
		dialer = unixContextDialer(net.Dialer{})

	default:
		return &endpointState{
			endpoint:   endpoint,
			connection: nil,
			err:        fmt.Errorf("invalid scheme %q", u.Scheme),
		}
	}

	klog.V(4).InfoS("Connecting to CRI endpoint", "Scheme", u.Scheme, "Path", u.Path)
	conn, err := grpc.DialContext(
		connCtx,
		u.Path,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithContextDialer(dialer),
		grpc.WithConnectParams(grpc.ConnectParams{
			MinConnectTimeout: dialTimeout,
			// Backoff configures the exponential backoff when establishing a new connection with DialContext.
			Backoff: backoffConfig,
		}),
		// WithDisableRetry disables retries (we have our own work queue with backoff) and retryThrottler.
		// We should not retry calls within the same sync loop and this is essential to avoid an infinite loop
		// between the sync loop context timing out and retryThrottler reaching a max delay higher than the timeout.
		grpc.WithDisableRetry(),
	)
	return &endpointState{
		endpoint:   endpoint,
		connection: conn,
		err:        err,
	}
}

func connectToEndpoints(ctx context.Context, endpoints []string) []*endpointState {
	// We need to keep the order, so we'll use an array instead of a channel.
	results := make([]*endpointState, len(endpoints))

	var wg sync.WaitGroup
	defer wg.Wait()
	for i := range endpoints {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			results[i] = connectToEndpoint(ctx, endpoints[i])
		}(i)
	}

	return results
}

func getConnection(ctx context.Context, endpoints []string) (conn *grpc.ClientConn, err error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("at least one cri endpoint is needed")
	}

	results := connectToEndpoints(ctx, endpoints)

	var successfulConns []*grpc.ClientConn
	var successfulEndpoints []string
	var errs []error
	for _, s := range results {
		if s.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.endpoint, s.err))
			continue
		}

		successfulConns = append(successfulConns, s.connection)
		successfulEndpoints = append(successfulEndpoints, s.endpoint)
	}

	if len(successfulConns) == 0 {
		return nil, apierrors.NewAggregate(errs)
	}

	klog.V(2).InfoS("Connected to CRI endpoint", "Successful", successfulEndpoints, "Other attempts", apierrors.NewAggregate(errs))

	return successfulConns[0], nil
}

type Client interface {
	Inspect(ctx context.Context, containerID string) (*ContainerStatus, error)
	Close() error
}

type client struct {
	conn   *grpc.ClientConn
	client pb.RuntimeServiceClient
}

func NewClient(ctx context.Context, endpoints []string) (*client, error) {
	conn, err := getConnection(ctx, endpoints)
	if err != nil {
		return nil, fmt.Errorf("can't create connection: %w", err)
	}

	return &client{
		conn:   conn,
		client: pb.NewRuntimeServiceClient(conn),
	}, nil
}

func (c *client) Close() error {
	return c.conn.Close()
}

type ContainerStatusInfoRuntimeSpecLinuxResourcesCPU struct {
	Shares int    `json:"shares"`
	Quota  int    `json:"quota"`
	Period int    `json:"period"`
	Cpus   string `json:"cpus"`
}

type ContainerStatusInfoRuntimeSpecLinuxResources struct {
	CPU ContainerStatusInfoRuntimeSpecLinuxResourcesCPU `json:"cpu"`
}

type ContainerStatusInfoRuntimeSpecLinux struct {
	Resources ContainerStatusInfoRuntimeSpecLinuxResources `json:"resources"`
}

type ContainerStatusInfoRuntimeSpec struct {
	Linux ContainerStatusInfoRuntimeSpecLinux `json:"linux"`
}

type ContainerStatusInfo struct {
	RuntimeSpec *ContainerStatusInfoRuntimeSpec `json:"runtimeSpec,omitempty"`
}

type ContainerStatus struct {
	Status *pb.ContainerStatus
	Info   ContainerStatusInfo
}

func (c *client) Inspect(ctx context.Context, containerID string) (*ContainerStatus, error) {
	r := &pb.ContainerStatusRequest{
		ContainerId: containerID,
		Verbose:     true,
	}
	resp, err := c.client.ContainerStatus(ctx, r)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil, NotFoundErr
		}
		return nil, err
	}

	cs := &ContainerStatus{
		Status: resp.Status,
	}

	if infoData, ok := resp.Info["info"]; ok {
		if err := json.Unmarshal([]byte(infoData), &cs.Info); err != nil {
			return nil, err
		}
	}

	return cs, nil
}

func unixContextDialer(d net.Dialer) func(ctx context.Context, addr string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		return d.DialContext(ctx, "unix", addr)
	}
}
