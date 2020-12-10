// Copyright (C) 2017 ScyllaDB

package fsm_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/util/fsm"
	"go.uber.org/atomic"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

const (
	Registration      fsm.State = "registration"
	DoctorAppointment fsm.State = "doctor_appointment"
	ApplyMedicine     fsm.State = "medicine"
	RequirePayment    fsm.State = "require_payment"

	ActionSuccess fsm.Event = "success"
)

var _ = Describe("FSM tests", func() {
	type patient struct {
		registrationDone bool
		doctorAnalyzed   bool
		medicineApplied  bool
		paymentDone      bool
	}

	var (
		ctx        context.Context
		hookCalled *atomic.Int64

		countingHook = func(ctx context.Context, currentState, nextState fsm.State, event fsm.Event) error {
			hookCalled.Inc()
			return nil
		}
	)

	BeforeEach(func() {
		ctx = context.Background()
		hookCalled = atomic.NewInt64(0)
	})

	It("Full transition", func() {
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
		}, countingHook)

		Expect(fsm.Transition(ctx)).To(Succeed())

		Expect(hookCalled.Load()).To(Equal(int64(3)))

		Expect(p.registrationDone).To(BeTrue())
		Expect(p.doctorAnalyzed).To(BeTrue())
		Expect(p.medicineApplied).To(BeTrue())
		Expect(p.paymentDone).To(BeTrue())
	})

	It("action failure", func() {
		fsm := fsm.New(Registration, fsm.StateTransitions{
			Registration: fsm.Transition{
				Action: func(ctx context.Context) (fsm.Event, error) {
					return fsm.NoOp, errors.New("fail!")
				},
			},
		}, countingHook)

		Expect(fsm.Transition(ctx)).To(HaveOccurred())
		Expect(hookCalled.Load()).To(Equal(int64(0)))
		Expect(fsm.Current()).To(Equal(Registration))
	})

	It("hook failure interrupts machine", func() {
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

		Expect(fsm.Transition(ctx)).To(HaveOccurred())
		Expect(fsm.Current()).To(Equal(Registration))
	})
})

func TestFSM(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"FSM Suite",
		[]Reporter{printer.NewlineReporter{}})
}
