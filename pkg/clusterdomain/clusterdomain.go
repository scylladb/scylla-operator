package clusterdomain

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

type DNSResolverInterface interface {
	LookupCNAME(ctx context.Context, name string) (string, error)
}

type DynamicClusterDomain struct {
	resolver DNSResolverInterface
}

func NewDynamicClusterDomain(resolver DNSResolverInterface) *DynamicClusterDomain {
	return &DynamicClusterDomain{
		resolver: resolver,
	}
}

func (dcd *DynamicClusterDomain) GetClusterDomain(ctx context.Context) (string, error) {
	const (
		kubernetesServiceHost = "kubernetes.default.svc"
	)

	cname, err := dcd.resolver.LookupCNAME(ctx, kubernetesServiceHost)
	if err != nil {
		return "", fmt.Errorf("can't lookup CNAME for %q: %w", kubernetesServiceHost, err)
	}

	clusterDomain := strings.TrimPrefix(cname, kubernetesServiceHost)
	clusterDomain = strings.Trim(clusterDomain, ".")

	if len(clusterDomain) == 0 {
		return "", fmt.Errorf("resolved CNAME after trimming is empty, resolved CNAME: %q", cname)
	}

	klog.V(4).InfoS("Discovered Kubernetes cluster domain", "ClusterDomain", clusterDomain)

	return clusterDomain, nil
}
