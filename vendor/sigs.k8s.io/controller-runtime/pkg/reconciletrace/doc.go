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

// Package reconciletrace records what a controller does and why: the watch events that enqueued a request, the
// enqueues the workqueue coalesced into one reconcile, the reconcile and its result, and the client calls the
// reconcile made.
//
// A controller records when controller.Options.TraceRecorder is set. Client calls are recorded by wrapping the
// client the reconciler uses with WrapClient; the calls are attributed to the reconcile through its context.
// When no recorder is set nothing is wrapped and nothing is recorded.
//
// The records are meant for offline review of controller behaviour, e.g. by the critic in the critic subpackage.
package reconciletrace
