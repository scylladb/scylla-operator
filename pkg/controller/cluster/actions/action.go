package actions

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Action represents a discrete action that a scylla rack can take.
// This can be a rack creation,
type Action interface {
	// Name returns the action's name.
	// Useful for unit-testing.
	Name() string

	// Execute will execute the action using the
	// ClusterController to read and write the
	// state of the system.
	Execute(ctx context.Context, s *State) error
}

// State contains tools to discover and modify
// the state of the world.
type State struct {
	// Dynamic client - uses caching for lookups
	client.Client
	// Uncached client
	kubeclient kubernetes.Interface
	// Recorder to create events
	recorder record.EventRecorder
}

func NewState(client client.Client, kubeclient kubernetes.Interface, recorder record.EventRecorder) *State {
	return &State{
		Client:     client,
		kubeclient: kubeclient,
		recorder:   recorder,
	}
}
