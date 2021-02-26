// Copyright (C) 2017 ScyllaDB

package fsm_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/util/fsm"
	"go.uber.org/atomic"
)

const (
	Registration      fsm.State = "registration"
	DoctorAppointment fsm.State = "doctor_appointment"
	ApplyMedicine     fsm.State = "medicine"
	RequirePayment    fsm.State = "require_payment"

	ActionSuccess fsm.Event = "success"
)

type patient struct {
	registrationDone bool
	doctorAnalyzed   bool
	medicineApplied  bool
	paymentDone      bool
}

func newCountingHook(counter *atomic.Int64) func(context.Context, fsm.State, fsm.State, fsm.Event) error {
	return func(context.Context, fsm.State, fsm.State, fsm.Event) error {
		counter.Inc()
		return nil
	}
}

func TestFSMFullTransition(t *testing.T) {
	ctx := context.Background()
	hookCalled := atomic.NewInt64(0)
	p := patient{}

	fsm := fsm.New(Registration, fsm.StateTransitions{
		Registration: fsm.Transition{
			Action: func(ctx context.Context) (fsm.Event, error) {
				p.registrationDone = true
				return ActionSuccess, nil
			},
			Events: map[fsm.Event]fsm.State{
				ActionSuccess: DoctorAppointment,
			},
		},
		DoctorAppointment: fsm.Transition{
			Action: func(ctx context.Context) (fsm.Event, error) {
				p.doctorAnalyzed = true
				return ActionSuccess, nil
			},
			Events: map[fsm.Event]fsm.State{
				ActionSuccess: ApplyMedicine,
			},
		},
		ApplyMedicine: fsm.Transition{
			Action: func(ctx context.Context) (fsm.Event, error) {
				p.medicineApplied = true
				return ActionSuccess, nil
			},
			Events: map[fsm.Event]fsm.State{
				ActionSuccess: RequirePayment,
			},
		},
		RequirePayment: fsm.Transition{
			Action: func(ctx context.Context) (fsm.Event, error) {
				p.paymentDone = true
				return fsm.NoOp, nil
			},
			Events: map[fsm.Event]fsm.State{
				ActionSuccess: DoctorAppointment,
			},
		},
	}, newCountingHook(hookCalled))

	err := fsm.Transition(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = fsm.Transition(ctx)
	n := hookCalled.Load()
	if n != int64(3) {
		t.Errorf("expected 3 hook calls, got %d", n)
	}

	if !p.registrationDone {
		t.Errorf("registration wasn't done")
	}

	if !p.doctorAnalyzed {
		t.Errorf("doctor didn't analyze")
	}
	if !p.medicineApplied {
		t.Errorf("medicine wasn't applied")
	}
	if !p.paymentDone {
		t.Errorf("payment wasn't doe")
	}
}

func TestFSMActionFailure(t *testing.T) {
	ctx := context.Background()
	hookCalled := atomic.NewInt64(0)

	fsm := fsm.New(Registration, fsm.StateTransitions{
		Registration: fsm.Transition{
			Action: func(ctx context.Context) (fsm.Event, error) {
				return fsm.NoOp, errors.New("fail!")
			},
		},
	}, newCountingHook(hookCalled))

	err := fsm.Transition(ctx)
	if err == nil {
		t.Fatalf("expected error to occur")
	}

	err = fsm.Transition(ctx)
	n := hookCalled.Load()
	if n != int64(0) {
		t.Errorf("expected 0 hook calls, got %d", n)
	}

	current := fsm.Current()
	if current != Registration {
		t.Errorf("expected registration state, got %v", current)
	}
}

func TestFSMHookFailureInteruptsMachine(t *testing.T) {
	ctx := context.Background()

	fsm := fsm.New(Registration, fsm.StateTransitions{
		Registration: fsm.Transition{
			Action: func(ctx context.Context) (fsm.Event, error) {
				return ActionSuccess, nil
			},
			Events: map[fsm.Event]fsm.State{
				ActionSuccess: DoctorAppointment,
			},
		},
		DoctorAppointment: fsm.Transition{},
	}, func(ctx context.Context, currentState, nextState fsm.State, event fsm.Event) error {
		return errors.New("fail!")
	})

	err := fsm.Transition(ctx)
	if err == nil {
		t.Fatalf("expected error to occur")
	}

	current := fsm.Current()
	if current != Registration {
		t.Errorf("expected registration state, got %v", current)
	}
}
