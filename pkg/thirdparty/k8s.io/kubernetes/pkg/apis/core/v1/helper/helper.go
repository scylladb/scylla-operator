/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helper

import (
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

// AddOrUpdateTolerationInPodSpec tries to add a toleration to the toleration list in PodSpec.
// Returns true if something was updated, false otherwise.
func AddOrUpdateTolerationInPodSpec(spec *corev1.PodSpec, toleration *corev1.Toleration) bool {
	podTolerations := spec.Tolerations

	var newTolerations []corev1.Toleration
	updated := false
	for i := range podTolerations {
		if toleration.MatchToleration(&podTolerations[i]) {
			if apiequality.Semantic.DeepEqual(toleration, podTolerations[i]) {
				return false
			}
			newTolerations = append(newTolerations, *toleration)
			updated = true
			continue
		}

		newTolerations = append(newTolerations, podTolerations[i])
	}

	if !updated {
		newTolerations = append(newTolerations, *toleration)
	}

	spec.Tolerations = newTolerations
	return true
}
