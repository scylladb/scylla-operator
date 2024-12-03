package scylladbmonitoring

import (
	"fmt"
	"os"
	"testing"

	prometheusv1assets "github.com/scylladb/scylla-operator/assets/monitoring/prometheus/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func TestMain(m *testing.M) {
	m.Run()

	// We need to make sure that all prometheus rules (coming from scylladb monitoring) are wired.
	// Note that this can also mean we just lack coverage but both cases should be fixed.
	var errs []error
	for f, r := range prometheusv1assets.PrometheusRules.Get() {
		if !r.Accessed() {
			errs = append(errs, fmt.Errorf("prometheus rule %q has not been used in any test and may not be used in the codebase", f))
		}
	}

	err := utilerrors.NewAggregate(errs)
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}
