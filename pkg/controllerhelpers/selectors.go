package controllerhelpers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func FilterObjectMapByLabel[T metav1.Object](objects map[string]T, selector labels.Selector) map[string]T {
	res := map[string]T{}

	for name, obj := range objects {
		if selector.Matches(labels.Set(obj.GetLabels())) {
			res[name] = obj
		}
	}

	return res
}
