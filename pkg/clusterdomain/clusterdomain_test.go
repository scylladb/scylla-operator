package clusterdomain

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestNewDynamicClusterDomain(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                  string
		resolver              *fakeCNAMEResolver
		expectedClusterDomain string
		expectedErr           error
	}{
		{
			name: "cluster domain from resolved CNAME",
			resolver: newFakeCNAMEResolver(map[string]string{
				"kubernetes.default.svc": "kubernetes.default.svc.cluster.local",
			}),
			expectedClusterDomain: "cluster.local",
			expectedErr:           nil,
		},
		{
			name: "cluster domain is trimmed when CNAME has trailing dot",
			resolver: newFakeCNAMEResolver(map[string]string{
				"kubernetes.default.svc": "kubernetes.default.svc.cluster.local.",
			}),
			expectedClusterDomain: "cluster.local",
			expectedErr:           nil,
		},
		{
			name: "error when trimmed cluster domain is empty",
			resolver: newFakeCNAMEResolver(map[string]string{
				"kubernetes.default.svc": "kubernetes.default.svc",
			}),
			expectedClusterDomain: "",
			expectedErr:           fmt.Errorf("resolved CNAME after trimming is empty, resolved CNAME: %q", "kubernetes.default.svc"),
		},
		{
			name:                  "error host cannot be resolved",
			resolver:              newFakeCNAMEResolver(map[string]string{}),
			expectedClusterDomain: "",
			expectedErr:           fmt.Errorf("can't lookup CNAME for %q: %w", "kubernetes.default.svc", fmt.Errorf("no CNAME record for host %q", "kubernetes.default.svc")),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			dcd := NewDynamicClusterDomain(tc.resolver)
			gotClusterDomain, err := dcd.GetClusterDomain(ctx)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected %v error, got %v", tc.expectedErr, err)
			}
			if gotClusterDomain != tc.expectedClusterDomain {
				t.Errorf("expected %q cluster domain, got %q", tc.expectedClusterDomain, gotClusterDomain)
			}

		})
	}
}

type fakeCNAMEResolver struct {
	hostToCNAME map[string]string
}

func newFakeCNAMEResolver(hostToCNAME map[string]string) *fakeCNAMEResolver {
	return &fakeCNAMEResolver{
		hostToCNAME: hostToCNAME,
	}
}

func (r *fakeCNAMEResolver) LookupCNAME(_ context.Context, host string) (string, error) {
	cname, ok := r.hostToCNAME[host]
	if !ok {
		return "", fmt.Errorf("no CNAME record for host %q", host)
	}
	return cname, nil
}
