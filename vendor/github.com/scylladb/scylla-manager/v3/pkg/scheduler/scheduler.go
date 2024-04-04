// Copyright (C) 2017 ScyllaDB

package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/scylladb/scylla-manager/v3/pkg/util/retry"
)

// Properties are externally defined task parameters.
type Properties = any

// RunContext is a bundle of Context, Key, Properties and additional runtime
// information.
type RunContext[K comparable] struct {
	context.Context //nolint:containedctx
	Key             K
	Properties      Properties
	Retry           int8

	err error
}

func newRunContext[K comparable](key K, properties Properties, stop time.Time) (*RunContext[K], context.CancelFunc) {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if stop.IsZero() {
		ctx, cancel = context.WithCancel(context.Background())
	} else {
		ctx, cancel = context.WithDeadline(context.Background(), stop)
	}

	return &RunContext[K]{
		Context:    ctx,
		Key:        key,
		Properties: properties,
	}, cancel
}

// RunFunc specifies interface for key execution.
// When the provided context is cancelled function must return with
// context.Cancelled error, or an error caused by this error.
// Compatible functions can be passed to Scheduler constructor.
type RunFunc[K comparable] func(ctx RunContext[K]) error

// Trigger provides the next activation date.
// Implementations must return the same values for the same now parameter.
// A zero time can be returned to indicate no more executions.
type Trigger interface {
	Next(now time.Time) time.Time
}

// Details holds Properties, Trigger and auxiliary Key configuration.
type Details struct {
	Properties Properties
	Trigger    Trigger
	Backoff    retry.Backoff
	Window     Window
	Location   *time.Location
}

// Scheduler manages keys and triggers.
// A key uniquely identifies a scheduler task.
// There can be a single instance of a key scheduled or running at all times.
// Scheduler gets the next activation time for a key from a trigger.
// On key activation the RunFunc is called.
type Scheduler[K comparable] struct {
	now      func() time.Time
	run      RunFunc[K]
	listener Listener[K]
	timer    *time.Timer

	queue   *activationQueue[K]
	details map[K]Details
	running map[K]context.CancelFunc
	closed  bool
	mu      sync.Mutex

	wakeupCh chan struct{}
	wg       sync.WaitGroup
}

func NewScheduler[K comparable](now func() time.Time, run RunFunc[K], listener Listener[K]) *Scheduler[K] {
	return &Scheduler[K]{
		now:      now,
		run:      run,
		listener: listener,
		timer:    time.NewTimer(0),
		queue:    newActivationQueue[K](),
		details:  make(map[K]Details),
		running:  make(map[K]context.CancelFunc),
		wakeupCh: make(chan struct{}, 1),
	}
}

// Schedule updates properties and trigger of an existing key or adds a new key.
func (s *Scheduler[K]) Schedule(ctx context.Context, key K, d Details) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.details[key] = d
	// If running key will be scheduled when done in reschedule.
	if _, running := s.running[key]; running {
		return
	}

	now := s.now()
	if d.Location != nil {
		now = now.In(d.Location)
	}
	next := d.Trigger.Next(now)

	s.scheduleLocked(ctx, key, next, 0, nil, d.Window)
}

func (s *Scheduler[K]) reschedule(ctx *RunContext[K]) {
	key := ctx.Key

	s.mu.Lock()
	defer s.mu.Unlock()

	cancel, ok := s.running[key]
	if ok {
		cancel()
	}
	delete(s.running, key)

	if s.closed {
		return
	}
	d, ok := s.details[key]
	if !ok {
		return
	}

	now := s.now()
	if d.Location != nil {
		now = now.In(d.Location)
	}
	next := d.Trigger.Next(now)

	var (
		retno int8
		p     Properties
	)
	switch {
	case shouldContinue(ctx.err):
		next = now
		retno = ctx.Retry
		p = ctx.Properties
	case shouldRetry(ctx.err):
		if d.Backoff != nil {
			if b := d.Backoff.NextBackOff(); b != retry.Stop {
				next = now.Add(b)
				retno = ctx.Retry + 1
				p = ctx.Properties
				s.listener.OnRetryBackoff(ctx, key, b, retno)
			}
		}
	default:
		if d.Backoff != nil {
			d.Backoff.Reset()
		}
	}
	s.scheduleLocked(ctx, key, next, retno, p, d.Window)
}

func shouldContinue(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

func shouldRetry(err error) bool {
	return !(err == nil || errors.Is(err, context.Canceled) || retry.IsPermanent(err))
}

func (s *Scheduler[K]) scheduleLocked(ctx context.Context, key K, next time.Time, retno int8, p Properties, w Window) {
	if next.IsZero() {
		s.listener.OnNoTrigger(ctx, key)
		s.unscheduleLocked(key)
		return
	}

	begin, end := w.Next(next)

	s.listener.OnSchedule(ctx, key, begin, end, retno)
	a := Activation[K]{Key: key, Time: begin, Retry: retno, Properties: p, Stop: end}
	if s.queue.Push(a) {
		s.wakeup()
	}
}

// Unschedule cancels schedule of a key. It does not stop an active run.
func (s *Scheduler[K]) Unschedule(ctx context.Context, key K) {
	s.listener.OnUnschedule(ctx, key)
	s.mu.Lock()
	s.unscheduleLocked(key)
	s.mu.Unlock()
}

func (s *Scheduler[K]) unscheduleLocked(key K) {
	delete(s.details, key)
	if s.queue.Remove(key) {
		s.wakeup()
	}
}

// Trigger immediately runs a scheduled key.
// If key is already running the call will have no effect and true is returned.
// If key is not scheduled the call will have no effect and false is returned.
func (s *Scheduler[K]) Trigger(ctx context.Context, key K) bool {
	s.mu.Lock()
	if _, running := s.running[key]; running {
		s.mu.Unlock()
		s.listener.OnTrigger(ctx, key, true)
		return true
	}

	if s.queue.Remove(key) {
		s.wakeup()
	}
	_, ok := s.details[key]
	var runCtx *RunContext[K]
	if ok {
		runCtx = s.newRunContextLocked(Activation[K]{Key: key})
	}
	s.mu.Unlock()

	s.listener.OnTrigger(ctx, key, ok)
	if ok {
		s.asyncRun(runCtx)
	}
	return ok
}

// Stop notifies RunFunc to stop by cancelling the context.
func (s *Scheduler[K]) Stop(ctx context.Context, key K) {
	s.listener.OnStop(ctx, key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if cancel, ok := s.running[key]; ok {
		cancel()
	}
}

// Close makes Start function exit, stops all runs, call Wait to wait for the
// runs to return.
// It returns two sets of keys the running that were canceled and pending that
// were scheduled to run.
func (s *Scheduler[K]) Close() (running, pending []K) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	s.wakeup()
	for k, cancel := range s.running {
		running = append(running, k)
		cancel()
	}
	for _, a := range s.queue.h {
		pending = append(pending, a.Key)
	}
	return
}

// Wait waits for runs to return call after Close.
func (s *Scheduler[_]) Wait() {
	s.wg.Wait()
}

// Activations returns activation information for given keys.
func (s *Scheduler[K]) Activations(keys ...K) []Activation[K] {
	pos := make(map[K]int, len(keys))
	for i, k := range keys {
		pos[k] = i
	}
	r := make([]Activation[K], len(keys))
	s.mu.Lock()
	for _, a := range s.queue.h {
		if i, ok := pos[a.Key]; ok {
			r[i] = a
		}
	}
	s.mu.Unlock()
	return r
}

// Start is the scheduler main loop.
func (s *Scheduler[_]) Start(ctx context.Context) {
	s.listener.OnSchedulerStart(ctx)

	for {
		var d time.Duration
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			break
		}
		top, ok := s.queue.Top()
		s.mu.Unlock()

		if !ok {
			d = -1
		} else {
			d = s.activateIn(top)
		}
		s.listener.OnSleep(ctx, top.Key, d)
		if !s.sleep(ctx, d) {
			if ctx.Err() != nil {
				break
			}
			continue
		}

		s.mu.Lock()
		a, ok := s.queue.Pop()
		if a.Key != top.Key {
			if ok {
				s.queue.Push(a)
			}
			s.mu.Unlock()
			continue
		}
		runCtx := s.newRunContextLocked(a)
		s.mu.Unlock()

		s.asyncRun(runCtx)
	}

	s.listener.OnSchedulerStop(ctx)
}

func (s *Scheduler[K]) activateIn(a Activation[K]) time.Duration {
	d := a.Sub(s.now())
	if d < 0 {
		d = 0
	}
	return d
}

// sleep waits for one of the following events: context is cancelled,
// duration expires (if d >= 0) or wakeup function is called.
// If d < 0 the timer is disabled.
// Returns true iff timer expired.
func (s *Scheduler[_]) sleep(ctx context.Context, d time.Duration) bool {
	if !s.timer.Stop() {
		select {
		case <-s.timer.C:
		default:
		}
	}

	if d == 0 {
		return true
	}

	var timer <-chan time.Time
	if d > 0 {
		s.timer.Reset(d)
		timer = s.timer.C
	}

	select {
	case <-ctx.Done():
		return false
	case <-s.wakeupCh:
		return false
	case <-timer:
		return true
	}
}

func (s *Scheduler[_]) wakeup() {
	select {
	case s.wakeupCh <- struct{}{}:
	default:
	}
}

func (s *Scheduler[K]) newRunContextLocked(a Activation[K]) *RunContext[K] {
	var p Properties
	if a.Properties != nil {
		p = a.Properties
	} else {
		p = s.details[a.Key].Properties
	}

	ctx, cancel := newRunContext(a.Key, p, a.Stop)
	ctx.Retry = a.Retry
	s.running[a.Key] = cancel
	return ctx
}

func (s *Scheduler[K]) asyncRun(ctx *RunContext[K]) {
	s.listener.OnRunStart(ctx)
	s.wg.Add(1)
	go func(ctx *RunContext[K]) {
		defer s.wg.Done()
		ctx.err = s.run(*ctx)
		s.onRunEnd(ctx)
		s.reschedule(ctx)
	}(ctx)
}

func (s *Scheduler[K]) onRunEnd(ctx *RunContext[K]) {
	err := ctx.err
	switch {
	case err == nil:
		s.listener.OnRunSuccess(ctx)
	case errors.Is(err, context.Canceled):
		s.listener.OnRunStop(ctx, err)
	case errors.Is(err, context.DeadlineExceeded):
		s.listener.OnRunWindowEnd(ctx, err)
	default:
		s.listener.OnRunError(ctx, err)
	}
}
