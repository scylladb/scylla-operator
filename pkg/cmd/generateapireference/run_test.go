package generateapireference

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

func TestIndexNestedProps(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                string
		props               map[string]apiextensionsv1.JSONSchemaProps
		expectedNestedProps map[string]apiextensionsv1.JSONSchemaProps
	}{
		{
			name: "simple object without properties",
			props: map[string]apiextensionsv1.JSONSchemaProps{
				"apiVersion": {
					Type: "string",
				},
				"metadata": {
					Type: "object",
				},
			},
			expectedNestedProps: map[string]apiextensionsv1.JSONSchemaProps{
				".metadata": {
					Type: "object",
				},
			},
		},
		{
			name: "nested objects with array containing an object",
			props: map[string]apiextensionsv1.JSONSchemaProps{
				"apiVersion": {
					Type: "string",
				},
				"spec": {
					Type: "object",
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"replicas": {
							Type: "integer",
						},
						"template": {
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"containers": {
									Type: "array",
									Items: &apiextensionsv1.JSONSchemaPropsOrArray{
										Schema: &apiextensionsv1.JSONSchemaProps{
											Type: "object",
											Properties: map[string]apiextensionsv1.JSONSchemaProps{
												"name": {
													Type: "string",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedNestedProps: map[string]apiextensionsv1.JSONSchemaProps{
				".spec": {
					Type: "object",
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"replicas": {
							Type: "integer",
						},
						"template": {
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"containers": {
									Type: "array",
									Items: &apiextensionsv1.JSONSchemaPropsOrArray{
										Schema: &apiextensionsv1.JSONSchemaProps{
											Type: "object",
											Properties: map[string]apiextensionsv1.JSONSchemaProps{
												"name": {
													Type: "string",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				".spec.template": {
					Type: "object",
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"containers": {
							Type: "array",
							Items: &apiextensionsv1.JSONSchemaPropsOrArray{
								Schema: &apiextensionsv1.JSONSchemaProps{
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"name": {
											Type: "string",
										},
									},
								},
							},
						},
					},
				},
				".spec.template.containers[]": {
					Type: "object",
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"name": {
							Type: "string",
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := IndexNestedProps(tc.props)
			if !apiequality.Semantic.DeepEqual(got, tc.expectedNestedProps) {
				t.Errorf("expected and got props differ:\n%s", cmp.Diff(tc.expectedNestedProps, got))
			}
		})
	}
}
