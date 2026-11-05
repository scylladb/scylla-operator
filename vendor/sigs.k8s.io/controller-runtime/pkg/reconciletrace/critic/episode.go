/*
Copyright 2026 The Kubernetes Authors.

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

package critic

import (
	"cmp"
	"slices"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconciletrace"
)

// Episode is the stretch of a trace one controller spent on one request: its enqueues, reconciles and the client
// calls of those reconciles, between two quiet gaps.
type Episode struct {
	Controller string
	Request    string
	// Records are ordered by ID.
	Records []reconciletrace.Record
}

// Start returns the time of the first record.
func (e *Episode) Start() time.Time {
	if len(e.Records) == 0 {
		return time.Time{}
	}

	return e.Records[0].Time
}

// End returns the time of the last record.
func (e *Episode) End() time.Time {
	if len(e.Records) == 0 {
		return time.Time{}
	}

	return e.Records[len(e.Records)-1].Time
}

// SplitEpisodes groups records by controller and request, ordered by ID, and splits each group where no record
// arrived for quietGap. Records attributed to no request, e.g. client calls made outside a reconcile, are dropped.
// A zero quietGap never splits.
func SplitEpisodes(records []reconciletrace.Record, quietGap time.Duration) []Episode {
	type key struct{ controller, request string }

	sorted := slices.Clone(records)
	slices.SortFunc(sorted, func(a, b reconciletrace.Record) int {
		return cmp.Compare(a.ID, b.ID)
	})

	var keys []key
	groups := map[key][]reconciletrace.Record{}
	for _, r := range sorted {
		if r.Controller == "" || r.Request == "" {
			continue
		}
		k := key{r.Controller, r.Request}
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], r)
	}

	var episodes []Episode
	for _, k := range keys {
		group := groups[k]
		start := 0
		for i := 1; i <= len(group); i++ {
			if i < len(group) && (quietGap == 0 || group[i].Time.Sub(group[i-1].Time) < quietGap) {
				continue
			}
			episodes = append(episodes, Episode{
				Controller: k.controller,
				Request:    k.request,
				Records:    group[start:i],
			})
			start = i
		}
	}

	return episodes
}
