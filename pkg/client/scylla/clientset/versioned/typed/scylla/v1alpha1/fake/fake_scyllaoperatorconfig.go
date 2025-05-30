// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	gentype "k8s.io/client-go/gentype"
)

// fakeScyllaOperatorConfigs implements ScyllaOperatorConfigInterface
type fakeScyllaOperatorConfigs struct {
	*gentype.FakeClientWithList[*v1alpha1.ScyllaOperatorConfig, *v1alpha1.ScyllaOperatorConfigList]
	Fake *FakeScyllaV1alpha1
}

func newFakeScyllaOperatorConfigs(fake *FakeScyllaV1alpha1) scyllav1alpha1.ScyllaOperatorConfigInterface {
	return &fakeScyllaOperatorConfigs{
		gentype.NewFakeClientWithList[*v1alpha1.ScyllaOperatorConfig, *v1alpha1.ScyllaOperatorConfigList](
			fake.Fake,
			"",
			v1alpha1.SchemeGroupVersion.WithResource("scyllaoperatorconfigs"),
			v1alpha1.SchemeGroupVersion.WithKind("ScyllaOperatorConfig"),
			func() *v1alpha1.ScyllaOperatorConfig { return &v1alpha1.ScyllaOperatorConfig{} },
			func() *v1alpha1.ScyllaOperatorConfigList { return &v1alpha1.ScyllaOperatorConfigList{} },
			func(dst, src *v1alpha1.ScyllaOperatorConfigList) { dst.ListMeta = src.ListMeta },
			func(list *v1alpha1.ScyllaOperatorConfigList) []*v1alpha1.ScyllaOperatorConfig {
				return gentype.ToPointerSlice(list.Items)
			},
			func(list *v1alpha1.ScyllaOperatorConfigList, items []*v1alpha1.ScyllaOperatorConfig) {
				list.Items = gentype.FromPointerSlice(items)
			},
		),
		fake,
	}
}
