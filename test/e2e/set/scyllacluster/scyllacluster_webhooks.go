// Copyright (c) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
)

const warningHandlerContextKey = "warningHandlerContextKey"

type testWarningHandler struct {
}

// HandleWarningHeaderWithContext implements rest.WarningHandlerWithContext.
// It retrieves a pointer to a string from the context and updates it with the warning message.
// This allows for concurrent execution of tests with a shared client (enforced by the unfortunate framework implementation).
func (h testWarningHandler) HandleWarningHeaderWithContext(ctx context.Context, _ int, _ string, text string) {
	v := ctx.Value(warningHandlerContextKey)
	if warningPtr, ok := v.(*string); ok {
		*warningPtr = text
	}
}

var _ = g.Describe("ScyllaCluster's admission webhook", func() {
	f := framework.NewFramework("scyllacluster")

	warningHandler := &testWarningHandler{}
	f.AdminClient.Config.WarningHandlerWithContext = warningHandler

	type entry struct {
		modifierFuncs          []func(*scyllav1.ScyllaCluster)
		expectedErrMatcherFunc func(sc *scyllav1.ScyllaCluster) o.OmegaMatcher
		expectedWarning        string
	}

	const (
		sysctlDeprecationWarning = "spec.sysctls: deprecated; use NodeConfig's .spec.sysctls instead"
	)

	duplicateRacks := func(sc *scyllav1.ScyllaCluster) {
		// Add a duplicate rack to fail validation.
		sc.Spec.Datacenter.Racks = append(sc.Spec.Datacenter.Racks, *sc.Spec.Datacenter.Racks[0].DeepCopy())
	}

	setDeprecatedSysctls := func(sc *scyllav1.ScyllaCluster) {
		// Set a deprecated field to receive a warning on admission.
		sc.Spec.Sysctls = []string{
			// Use an invalid variable not to modify any actual kernel parameters.
			"scylla-operator.scylladb.com/dummy=1",
		}
	}

	g.DescribeTableSubtree("should respond", func(e *entry) {
		g.It("is created", func(ctx g.SpecContext) {
			var err error
			var warning string
			warningCtx := context.WithValue(ctx, warningHandlerContextKey, &warning)

			sc := f.GetDefaultScyllaCluster()
			// Set an explicit name to be able to fill in the expected error message on rejection.
			sc.Name = names.SimpleNameGenerator.GenerateName(sc.GenerateName)
			for _, f := range e.modifierFuncs {
				f(sc)
			}

			framework.By("Creating a ScyllaCluster")
			_, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(warningCtx, sc, metav1.CreateOptions{})
			o.Expect(err).To(e.expectedErrMatcherFunc(sc))
			o.Expect(warning).To(o.Equal(e.expectedWarning))
		})

		g.It("is updated", func(ctx g.SpecContext) {
			var err error

			framework.By("Creating a ScyllaCluster")
			sc := f.GetDefaultScyllaCluster()
			sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			scCopy := sc.DeepCopy()
			for _, f := range e.modifierFuncs {
				f(scCopy)
			}
			patch, err := controllerhelpers.GenerateMergePatch(sc, scCopy)
			o.Expect(err).NotTo(o.HaveOccurred())

			var warning string
			warningCtx := context.WithValue(ctx, warningHandlerContextKey, &warning)

			framework.By("Updating a ScyllaCluster")
			_, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
				warningCtx,
				sc.Name,
				types.MergePatchType,
				patch,
				metav1.PatchOptions{},
			)
			o.Expect(err).To(e.expectedErrMatcherFunc(scCopy))
			o.Expect(warning).To(o.Equal(e.expectedWarning))
		})
	},
		g.Entry("with acceptance and no warning when a valid ScyllaCluster", &entry{
			modifierFuncs: nil,
			expectedErrMatcherFunc: func(_ *scyllav1.ScyllaCluster) o.OmegaMatcher {
				return o.Not(o.HaveOccurred())
			},
			expectedWarning: "",
		}),
		g.Entry("with rejection and no warning when a ScyllaCluster with duplicated racks", &entry{
			modifierFuncs: []func(*scyllav1.ScyllaCluster){
				duplicateRacks,
			},
			expectedErrMatcherFunc: func(sc *scyllav1.ScyllaCluster) o.OmegaMatcher {
				return o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
					Status:  "Failure",
					Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaCluster.scylla.scylladb.com %q is invalid: spec.datacenter.racks[1].name: Duplicate value: "us-east-1a"`, sc.GetName()),
					Reason:  "Invalid",
					Details: &metav1.StatusDetails{
						Name:  sc.GetName(),
						Group: "scylla.scylladb.com",
						Kind:  "ScyllaCluster",
						UID:   "",
						Causes: []metav1.StatusCause{
							{
								Type:    "FieldValueDuplicate",
								Message: `Duplicate value: "us-east-1a"`,
								Field:   "spec.datacenter.racks[1].name",
							},
						},
					},
					Code: 422,
				}})
			},
			expectedWarning: "",
		}),
		g.Entry("with acceptance and a warning when a ScyllaCluster with deprecated sysctls", &entry{
			modifierFuncs: []func(*scyllav1.ScyllaCluster){
				setDeprecatedSysctls,
			},
			expectedErrMatcherFunc: func(_ *scyllav1.ScyllaCluster) o.OmegaMatcher {
				return o.Not(o.HaveOccurred())
			},
			expectedWarning: sysctlDeprecationWarning,
		}),
		g.Entry("with rejection and warning when a ScyllaCluster with duplicated racks and deprecated sysctls", &entry{
			modifierFuncs: []func(*scyllav1.ScyllaCluster){
				duplicateRacks,
				setDeprecatedSysctls,
			},
			expectedErrMatcherFunc: func(sc *scyllav1.ScyllaCluster) o.OmegaMatcher {
				return o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
					Status:  "Failure",
					Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaCluster.scylla.scylladb.com %q is invalid: spec.datacenter.racks[1].name: Duplicate value: "us-east-1a"`, sc.GetName()),
					Reason:  "Invalid",
					Details: &metav1.StatusDetails{
						Name:  sc.GetName(),
						Group: "scylla.scylladb.com",
						Kind:  "ScyllaCluster",
						UID:   "",
						Causes: []metav1.StatusCause{
							{
								Type:    "FieldValueDuplicate",
								Message: `Duplicate value: "us-east-1a"`,
								Field:   "spec.datacenter.racks[1].name",
							},
						},
					},
					Code: 422,
				}})
			},
			expectedWarning: sysctlDeprecationWarning,
		}),
	)
})
