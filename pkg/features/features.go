package features

import (
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"
)

const (
	// Feature gates should be listed in alphabetical, case-sensitive
	// (upper before any lower case character) order. This reduces the risk
	// of code conflicts because changes are more likely to be scattered
	// across the file.

	// AutomaticTLSCertificates enables automated provisioning and management of TLS certs.
	//
	// owner: @tnozicka
	// alpha: v1.8
	// beta: v1.11
	AutomaticTLSCertificates featuregate.Feature = "AutomaticTLSCertificates"
)

func init() {
	apimachineryutilruntime.Must(utilfeature.DefaultMutableFeatureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		AutomaticTLSCertificates: {
			Default:    true,
			PreRelease: featuregate.Beta,
		},
	}))
}
