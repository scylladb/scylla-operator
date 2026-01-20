package debounce

import (
	"sync"
	"time"
)

const (
	RingRefreshDebounceTime = 1 * time.Second
)

// debounces requests to call a refresh function (currently used for ring refresh). It also supports triggering a refresh immediately.
type RefreshDebouncer struct {
	broadcaster  *errorBroadcaster
	timer        *time.Timer
	refreshNowCh chan struct{}
	quit         chan struct{}
	refreshFn    func() error
	interval     time.Duration
	mu           sync.Mutex
	stopped      bool
}

func NewRefreshDebouncer(interval time.Duration, refreshFn func() error) *RefreshDebouncer {
	d := &RefreshDebouncer{
		stopped:      false,
		broadcaster:  nil,
		refreshNowCh: make(chan struct{}, 1),
		quit:         make(chan struct{}),
		interval:     interval,
		timer:        time.NewTimer(interval),
		refreshFn:    refreshFn,
	}
	d.timer.Stop()
	go d.flusher()
	return d
}

// debounces a request to call the refresh function
func (d *RefreshDebouncer) Debounce() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return
	}
	d.timer.Reset(d.interval)
}

// requests an immediate refresh which will cancel pending refresh requests
func (d *RefreshDebouncer) RefreshNow() <-chan error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.broadcaster == nil {
		d.broadcaster = newErrorBroadcaster()
		select {
		case d.refreshNowCh <- struct{}{}:
		default:
			// already a refresh pending
		}
	}
	return d.broadcaster.newListener()
}

func (d *RefreshDebouncer) flusher() {
	for {
		select {
		case <-d.refreshNowCh:
		case <-d.timer.C:
		case <-d.quit:
		}
		d.mu.Lock()
		if d.stopped {
			if d.broadcaster != nil {
				d.broadcaster.stop()
				d.broadcaster = nil
			}
			d.timer.Stop()
			d.mu.Unlock()
			return
		}

		// make sure both request channels are cleared before we refresh
		select {
		case <-d.refreshNowCh:
		default:
		}

		d.timer.Stop()
		select {
		case <-d.timer.C:
		default:
		}

		curBroadcaster := d.broadcaster
		d.broadcaster = nil
		d.mu.Unlock()

		err := d.refreshFn()
		if curBroadcaster != nil {
			curBroadcaster.broadcast(err)
		}
	}
}

func (d *RefreshDebouncer) Stop() {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return
	}
	d.stopped = true
	d.mu.Unlock()
	d.quit <- struct{}{} // sync with flusher
	close(d.quit)
}

// broadcasts an error to multiple channels (listeners)
type errorBroadcaster struct {
	listeners []chan<- error
	mu        sync.Mutex
}

func newErrorBroadcaster() *errorBroadcaster {
	return &errorBroadcaster{
		listeners: nil,
		mu:        sync.Mutex{},
	}
}

func (b *errorBroadcaster) newListener() <-chan error {
	ch := make(chan error, 1)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners = append(b.listeners, ch)
	return ch
}

func (b *errorBroadcaster) broadcast(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	curListeners := b.listeners
	if len(curListeners) > 0 {
		b.listeners = nil
	} else {
		return
	}

	for _, listener := range curListeners {
		listener <- err
		close(listener)
	}
}

func (b *errorBroadcaster) stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.listeners) == 0 {
		return
	}
	for _, listener := range b.listeners {
		close(listener)
	}
	b.listeners = nil
}
