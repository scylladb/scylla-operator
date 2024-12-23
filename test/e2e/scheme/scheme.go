package scheme

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	Scheme = runtime.NewScheme()
	Codecs = serializer.NewCodecFactory(Scheme, serializer.EnableStrict)
)

func DefaultJSONEncoder() runtime.Encoder {
	return unstructured.NewJSONFallbackEncoder(Codecs.LegacyCodec(Scheme.PrioritizedVersionsAllGroups()...))
}
