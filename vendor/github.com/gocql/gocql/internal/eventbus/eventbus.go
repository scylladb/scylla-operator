/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package eventbus provides a generic event processing system based on channels.
// It allows multiple subscribers to receive events from a single input channel,
// with optional filtering and configurable buffer sizes.
package eventbus

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrAlreadyStarted is returned when Start is called on an already running EventBus
	ErrAlreadyStarted = errors.New("eventbus: already started")
	// ErrNotStarted is returned when operations are attempted on a non-started EventBus
	ErrNotStarted = errors.New("eventbus: not started")
	// ErrAlreadyStopped is returned when Stop is called on an already stopped EventBus
	ErrAlreadyStopped = errors.New("eventbus: already stopped")
	// ErrSubscriberNotFound is returned when unsubscribing a non-existent subscriber
	ErrSubscriberNotFound = errors.New("eventbus: subscriber not found")
)

// FilterFunc is a function type that filters events.
// It returns true if the event should be sent to the subscriber, false otherwise.
type FilterFunc[T any] func(T) bool

// subscriber represents a single subscriber to the event bus
type subscriber[T any] struct {
	ch     chan T
	filter FilterFunc[T]
	name   string
	id     int
}

// Subscriber provides access to events and control over the subscription
type Subscriber[T any] struct {
	ch   <-chan T
	eb   *EventBus[T]
	name string
	id   int
}

// Events returns the channel to receive events from
func (s *Subscriber[T]) Events() <-chan T {
	return s.ch
}

// Stop unsubscribes and closes the subscriber's channel
func (s *Subscriber[T]) Stop() error {
	return s.eb.remove(s)
}

type status uint8

const (
	statusInitialized status = iota
	statusStarted
	statusStopped
)

// EventBus manages event distribution to multiple subscribers
type EventBus[T any] struct {
	logger       StdLogger
	input        chan T
	closedSignal chan struct{}
	subscribers  []*subscriber[T]
	wg           sync.WaitGroup
	cfg          EventBusConfig
	mu           sync.RWMutex
	status       status
}

type StdLogger interface {
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

type EventBusConfig struct {
	// Size of the event queue, when queue gets more events than queue events are getting dropped.
	InputEventsQueueSize int
}

// New creates a new EventBus with the specified input channel buffer size.
// The EventBus must be status with Start() before it begins processing events.
func New[T any](cfg EventBusConfig, logger StdLogger) *EventBus[T] {
	return &EventBus[T]{
		cfg:          cfg,
		logger:       logger,
		input:        make(chan T, cfg.InputEventsQueueSize),
		closedSignal: make(chan struct{}, 1),
		status:       statusInitialized,
	}
}

// Start begins processing events from the input channel and distributing them to subscribers.
// Returns ErrAlreadyStarted if the EventBus is already running.
func (eb *EventBus[T]) Start() error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	switch eb.status {
	case statusStarted:
		return ErrAlreadyStarted
	case statusStopped:
		return ErrAlreadyStopped
	default:
	}

	eb.status = statusStarted
	eb.wg.Add(1)

	go eb.run()

	return nil
}

// Stop halts event processing and closes all subscriber channels.
// It waits for the processing goroutine to finish.
// Returns ErrNotStarted if the EventBus was never started, or ErrAlreadyStopped if already stopped.
func (eb *EventBus[T]) Stop() error {
	eb.mu.Lock()

	defer eb.mu.Unlock()
	switch eb.status {
	case statusStopped:
		return ErrAlreadyStopped
	case statusInitialized:
		return ErrNotStarted
	default:
		eb.status = statusStopped
	}
	close(eb.closedSignal)
	// Wait for the run goroutine to finish
	eb.mu.Unlock()
	eb.wg.Wait()
	eb.mu.Lock()

	for _, sub := range eb.subscribers {
		close(sub.ch)
	}
	eb.subscribers = nil
	return nil
}

// PublishEvent sends an event onto the bus. If the input buffer is full the
// event is dropped to avoid blocking publishers.
func (eb *EventBus[T]) PublishEvent(e T) bool {
	select {
	case eb.input <- e:
		return true
	default:
		return false
	}
}

// PublishEventBlocking sends an event onto the bus. If the input buffer is full is blocks, until event is published
func (eb *EventBus[T]) PublishEventBlocking(e T) {
	select {
	case eb.input <- e:
	}
}

// Subscribe adds a new subscriber to the event bus.
// name: unique identifier for the subscriber
// queueSize: buffer size for the subscriber's channel (must be >= 0)
// filter: optional filter function (can be nil to receive all events)
//
// Returns a Subscriber instance that provides access to events and a Stop method.
func (eb *EventBus[T]) Subscribe(name string, queueSize int, filter FilterFunc[T]) *Subscriber[T] {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	sub := &subscriber[T]{
		name:   name,
		ch:     make(chan T, queueSize),
		filter: filter,
	}
	eb.subscribers = append(eb.subscribers, sub)

	return &Subscriber[T]{
		ch:   sub.ch,
		name: name,
		eb:   eb,
	}
}

// Unsubscribe removes a subscriber from the event bus and closes its channel.
// Returns ErrSubscriberNotFound if the subscriber doesn't exist.
func (eb *EventBus[T]) remove(s *Subscriber[T]) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	subID := -1

	for id, sub := range eb.subscribers {
		if s.ch == sub.ch {
			subID = id
			close(sub.ch)
		}
	}

	if subID == -1 {
		return ErrSubscriberNotFound
	}

	eb.subscribers = append(eb.subscribers[0:subID], eb.subscribers[subID+1:]...)
	return nil
}

// SubscriberCount returns the current number of active subscribers
func (eb *EventBus[T]) SubscriberCount() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.subscribers)
}

// run is the main event processing loop
func (eb *EventBus[T]) run() {
	defer eb.wg.Done()

	for {
		select {
		case <-eb.closedSignal:
			return
		case event, ok := <-eb.input:
			if !ok {
				if eb.logger == nil {
					eb.logger.Printf("eventbus channel has been closed, it should not have happened, report the bug please.")
				}
				return
			}
			eb.distribute(event)
		}
	}
}

// distribute sends an event to all matching subscribers
func (eb *EventBus[T]) distribute(event T) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for _, sub := range eb.subscribers {
		if sub.filter != nil && !sub.filter(event) {
			continue
		}

		// Non-blocking send to avoid slow subscribers blocking the bus
		select {
		case sub.ch <- event:
		default:
			if eb.logger != nil {
				eb.logger.Printf("eventbus: dropped event for subscriber %s, make sure it is running and update it's channel size\n", sub.name)
			}
		}
	}
}

// SubscribeWithContext subscribes with a context that can cancel the subscription.
// The subscriber will be automatically unsubscribed when the context is cancelled.
// Returns a Subscriber instance and an error if subscription fails.
func (eb *EventBus[T]) SubscribeWithContext(ctx context.Context, name string, chanSize int, filter FilterFunc[T]) *Subscriber[T] {
	sub := eb.Subscribe(name, chanSize, filter)

	// Start a goroutine to handle context cancellation
	go func() {
		<-ctx.Done()
		_ = eb.remove(sub) // Ignore error if already unsubscribed
	}()

	return sub
}

// String returns a string representation of the EventBus for debugging
func (eb *EventBus[T]) String() string {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return fmt.Sprintf("EventBus{subscribers: %d, status: %v}", len(eb.subscribers), eb.status)
}
