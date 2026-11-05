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
	"context"
	"sync"
	"sync/atomic"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconciletrace"
)

// OnlineOptions configure an OnlineReviewer.
type OnlineOptions struct {
	// QuietGap ends an episode once its request has had no record for this long. Defaults to 30 seconds.
	QuietGap time.Duration
	// QueueSize bounds the episodes waiting for review; episodes ending while it's full are dropped. Defaults to 16.
	QueueSize int
	// OnReport receives every review. It runs on the reviewer's single worker.
	OnReport func(*Report, error)
}

// OnlineReviewer is a reconciletrace.Recorder that reviews each episode once its request goes quiet. Recording only
// appends to memory; the reviews run one at a time on a worker of their own, so a slow model never slows the
// controller down.
type OnlineReviewer struct {
	critic *Critic
	opts   OnlineOptions

	mu      sync.Mutex
	stopped bool
	pending map[episodeKey]*pendingEpisode

	work    chan Episode
	done    chan struct{}
	dropped atomic.Int64
}

type episodeKey struct {
	controller, request string
}

type pendingEpisode struct {
	records []reconciletrace.Record
	timer   *time.Timer
	// gen tells a timer that fired for an older generation of the episode, after a record reset it.
	gen int
}

var _ reconciletrace.Recorder = &OnlineReviewer{}

// NewOnlineReviewer starts a reviewer. Stop it to review the episodes still open and release its worker.
func NewOnlineReviewer(ctx context.Context, c *Critic, opts OnlineOptions) *OnlineReviewer {
	if opts.QuietGap <= 0 {
		opts.QuietGap = 30 * time.Second
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = 16
	}
	if opts.OnReport == nil {
		opts.OnReport = func(*Report, error) {}
	}

	o := &OnlineReviewer{
		critic:  c,
		opts:    opts,
		pending: map[episodeKey]*pendingEpisode{},
		work:    make(chan Episode, opts.QueueSize),
		done:    make(chan struct{}),
	}

	go func() {
		defer close(o.done)
		for e := range o.work {
			report, err := o.critic.Review(ctx, &e)
			o.opts.OnReport(report, err)
		}
	}()

	return o
}

// Record implements reconciletrace.Recorder.
func (o *OnlineReviewer) Record(r reconciletrace.Record) {
	if r.Controller == "" || r.Request == "" {
		return
	}
	k := episodeKey{r.Controller, r.Request}

	o.mu.Lock()
	defer o.mu.Unlock()

	if o.stopped {
		return
	}

	p, ok := o.pending[k]
	if !ok {
		p = &pendingEpisode{}
		o.pending[k] = p
	}
	p.records = append(p.records, r)
	p.gen++
	gen := p.gen
	if p.timer != nil {
		p.timer.Stop()
	}
	p.timer = time.AfterFunc(o.opts.QuietGap, func() {
		o.close(k, gen)
	})
}

// close hands the episode of k to the worker, unless a record arrived after the timer of gen was set. It sends under
// the lock, so that Stop can't close the queue in between.
func (o *OnlineReviewer) close(k episodeKey, gen int) {
	o.mu.Lock()
	defer o.mu.Unlock()

	p, ok := o.pending[k]
	if !ok || p.gen != gen || o.stopped {
		return
	}
	delete(o.pending, k)

	e := SplitEpisodes(p.records, 0)
	for i := range e {
		select {
		case o.work <- e[i]:
		default:
			o.dropped.Add(1)
		}
	}
}

// Dropped returns the number of episodes dropped because the review queue was full.
func (o *OnlineReviewer) Dropped() int64 {
	return o.dropped.Load()
}

// Stop reviews the episodes still open, waits for every review to finish and stops the worker. Records arriving
// later are dropped.
func (o *OnlineReviewer) Stop() {
	o.mu.Lock()
	if o.stopped {
		o.mu.Unlock()
		<-o.done
		return
	}
	o.stopped = true
	pending := o.pending
	o.pending = nil
	o.mu.Unlock()

	for _, p := range pending {
		p.timer.Stop()
		for _, e := range SplitEpisodes(p.records, 0) {
			o.work <- e
		}
	}
	close(o.work)
	<-o.done
}
