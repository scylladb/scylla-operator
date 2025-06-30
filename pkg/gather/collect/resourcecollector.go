package collect

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ResourceCollector interface {
	Collect(ctx context.Context, u *unstructured.Unstructured, resourceInfo *ResourceInfo) error
}
