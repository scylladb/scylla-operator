// Copyright (C) 2021 ScyllaDB

package cri

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog/v2"
)

const (
	dialTimeout = time.Second
)

var (
	runtimeEndpoints = []string{"/var/run/dockershim.sock", "/run/containerd/containerd.sock", "/run/crio/crio.sock"}

	NotFoundErr = fmt.Errorf("not found")
)

type Client interface {
	Inspect(ctx context.Context, containerID string) (*ContainerStatus, error)
	Close() error
}

type client struct {
	conn   *grpc.ClientConn
	client pb.RuntimeServiceClient
}

func NewClient(ctx context.Context, endpointPrefix string) (*client, error) {
	conn, err := getConnection(ctx, endpointPrefix, runtimeEndpoints, dialTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "connect")
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

func getConnection(ctx context.Context, endpointPrefix string, endpoints []string, timeout time.Duration) (conn *grpc.ClientConn, err error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("endpoint is not set")
	}
	for indx, endpoint := range endpoints {
		addr := endpointPrefix + endpoint
		klog.V(4).InfoS("connecting to container runtime service", "address", addr, "timeout", timeout)
		connCtx, connCtxCancel := context.WithTimeout(ctx, timeout)
		defer connCtxCancel()
		conn, err = grpc.DialContext(connCtx, addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithContextDialer(unixContextDialer(net.Dialer{})))
		if err != nil {
			if indx == len(endpoints)-1 {
				return nil, err
			}
			klog.V(4).InfoS("failed to connect to container runtime service", "address", addr, "error", err)
		} else {
			klog.V(2).InfoS("connected successfully using endpoint", "address", addr)
			break
		}
	}
	return conn, nil
}
