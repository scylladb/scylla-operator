// Copyright (c) 2024 ScyllaDB.

package kubelet

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	podresourcesv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
)

type PodResourcesClient interface {
	List(ctx context.Context) ([]*podresourcesv1.PodResources, error)
	Close() error
}

func NewPodResourcesClient(ctx context.Context, endpoint string) (*PodResourcesClientImpl, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint %q: %w", endpoint, err)
	}

	var dialer func(context.Context, string) (net.Conn, error)
	switch u.Scheme {
	case "unix":
		dialer = func(ctx context.Context, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", addr)
		}

	default:
		return nil, fmt.Errorf("unsupported endpoint scheme %q", u.Scheme)
	}

	conn, err := grpc.DialContext(
		ctx,
		u.Path,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithContextDialer(dialer),
	)
	if err != nil {
		return nil, fmt.Errorf("can't dial to %q: %w", endpoint, err)
	}

	return &PodResourcesClientImpl{
		conn:   conn,
		client: podresourcesv1.NewPodResourcesListerClient(conn),
	}, nil
}

type PodResourcesClientImpl struct {
	conn   *grpc.ClientConn
	client podresourcesv1.PodResourcesListerClient
}

func (c *PodResourcesClientImpl) List(ctx context.Context) ([]*podresourcesv1.PodResources, error) {
	resp, err := c.client.List(ctx, &podresourcesv1.ListPodResourcesRequest{})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("can't get pod resources: %w", err)
	}

	return resp.PodResources, nil
}

func (c *PodResourcesClientImpl) Close() error {
	return c.conn.Close()
}
