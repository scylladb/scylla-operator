// Copyright (C) 2017 ScyllaDB

package fsm

import (
	"context"

	"github.com/pkg/errors"
)

// ErrEventRejected is the error returned when the state machine cannot process
// an event in the state that it is in.
var ErrEventRejected = errors.New("event rejected")

const (
	// NoOp represents a no-op event. State machine stops when this event is emitted.
	NoOp Event = "NoOp"
)

// State represents an extensible state type in the state machine.
type State string

// Event represents an extensible event type in the state machine.
type Event string

// Action represents the action to be executed in a given state.
type Action func(ctx context.Context) (Event, error)

// Events represents a mapping of events and states.
type Events map[Event]State

// Transition binds a state with an action and a set of events it can handle.
type Transition struct {
	Action Action
	Events Events
}

// Hook is called on each state machine transition.
type Hook func(ctx context.Context, currentState, nextState State, event Event) error

// StateTransitions represents a mapping of states and their implementations.
type StateTransitions map[State]Transition

// StateMachine represents the state machine.
type StateMachine struct {
	// Current represents the current state.
	current State

	// StateTransitions holds the configuration of states and events handled by the state machine.
	stateTransitions StateTransitions

	// TransitionHook is called on every state transition.
	transitionHook Hook
}

// New returns initialized state machine.
func New(state State, stateTransitions StateTransitions, hook Hook) *StateMachine {
	return &StateMachine{
		current:          state,
		stateTransitions: stateTransitions,
		transitionHook:   hook,
	}
}

// getNextState returns the next state for the event given the machine's current
// state, or an error if the event can't be handled in the given state.
func (s *StateMachine) getNextState(event Event) (State, error) {
	if transition, ok := s.stateTransitions[s.current]; ok {
		if transition.Events != nil {
			if next, ok := transition.Events[event]; ok {
				return next, nil
			}
		}
	}
	return s.current, ErrEventRejected
}

// Transition triggers current state action and sends event to the state machine.
func (s *StateMachine) Transition(ctx context.Context) error {
	// Pick next transition according to current state.
	transition := s.stateTransitions[s.current]
	event, err := transition.Action(ctx)
	if err != nil {
		return err
	}
	if event == NoOp {
		return nil
	}

	for {
		// Determine the next state for the event given the machine's current state.
		nextState, err := s.getNextState(event)
		if err != nil {
			return errors.Wrapf(ErrEventRejected, "rejected %s", err.Error())
		}

		// Identify the state definition for the next state.
		nextTransition, ok := s.stateTransitions[nextState]
		if !ok || nextTransition.Action == nil {
			return errors.Wrapf(ErrEventRejected, "unknown transition %q for event %q", nextState, event)
		}

		if s.transitionHook != nil {
			if err := s.transitionHook(ctx, s.current, nextState, event); err != nil {
				return err
			}
		}
		// Transition over to the next state.
		s.current = nextState

		// Execute the next state's action and loop over again if the event returned
		// is not a no-op.
		nextEvent, err := nextTransition.Action(ctx)
		if err != nil {
			return err
		}
		if nextEvent == NoOp {
			return nil
		}
		event = nextEvent
	}
}

// Current return current state machine state.
func (s *StateMachine) Current() State {
	return s.current
}
