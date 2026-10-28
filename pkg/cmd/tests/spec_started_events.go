// Copyright (c) 2026 ScyllaDB.

package tests

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/onsi/ginkgo/v2/types"
	"github.com/scylladb/scylla-operator/pkg/thirdparty/github.com/onsi/ginkgo/v2/exposedinternal/parallel_support"
	"k8s.io/klog/v2"
)

// Ginkgo's parallel protocol only reports specs once they finish. To tell which specs are in flight, every parallel
// process sends an event to the parent process when it starts a spec, through the parallel server's output channel.

// specStartedEventKind marks the events that parallel processes send to the parent process through the parallel
// server's output channel when they start a spec.
const specStartedEventKind = "scylla-operator-tests/spec-started"

type specStartedEvent struct {
	Kind     string   `json:"kind"`
	Process  int      `json:"process"`
	Text     string   `json:"text"`
	Location string   `json:"location,omitempty"`
	Labels   []string `json:"labels,omitempty"`
}

// newSpecStartedNotifier connects to the parallel server and returns a function that tells the parent process about
// every spec that is about to run.
// It's meant to be called from a ReportBeforeEach node in a parallel process.
func newSpecStartedNotifier(serverAddress string) (func(types.SpecReport), error) {
	client := parallel_support.NewClient(serverAddress)
	if !client.Connect() {
		return nil, fmt.Errorf("can't connect to the parallel server at %q", serverAddress)
	}

	return func(report types.SpecReport) {
		// Specs that are filtered out or otherwise won't run go through ReportBeforeEach too.
		if report.State.Is(types.SpecStateSkipped | types.SpecStatePending | types.SpecStateFailureStates) {
			return
		}

		data, err := json.Marshal(specStartedEvent{
			Kind:     specStartedEventKind,
			Process:  report.ParallelProcess,
			Text:     specText(report),
			Location: report.LeafNodeLocation.String(),
			Labels:   report.Labels(),
		})
		if err != nil {
			klog.ErrorS(err, "Can't encode the spec started event")
			return
		}

		_, err = client.Write(data)
		if err != nil {
			klog.ErrorS(err, "Can't send the spec started event")
		}
	}, nil
}

// specStartedReceiver is the parallel server's output destination in the parent process. It hands the events about
// specs starting to handle, and anything else that comes through the channel to passthrough, as it is.
type specStartedReceiver struct {
	handle      func(specStartedEvent)
	passthrough func([]byte) (int, error)
}

var _ io.Writer = specStartedReceiver{}

func newSpecStartedReceiver(handle func(specStartedEvent), passthrough func([]byte) (int, error)) io.Writer {
	return specStartedReceiver{
		handle:      handle,
		passthrough: passthrough,
	}
}

func (r specStartedReceiver) Write(p []byte) (int, error) {
	// Every write carries one whole message of a parallel process.
	event := specStartedEvent{}
	err := json.Unmarshal(p, &event)
	if err != nil || event.Kind != specStartedEventKind {
		return r.passthrough(p)
	}

	r.handle(event)
	return len(p), nil
}
