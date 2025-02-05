package debounce

import (
	"sync"
	"sync/atomic"
)

// SimpleDebouncer is are tool for queuing immutable functions calls. It provides:
// 1. Blocking simultaneous calls
// 2. If there is no running call and no waiting call, then the current call go through
// 3. If there is running call and no waiting call, then the current call go waiting
// 4. If there is running call and waiting call, then the current call are voided
type SimpleDebouncer struct {
	m     sync.Mutex
	count atomic.Int32
}

// NewSimpleDebouncer creates a new SimpleDebouncer.
func NewSimpleDebouncer() *SimpleDebouncer {
	return &SimpleDebouncer{}
}

// Debounce attempts to execute the function if the logic of the SimpleDebouncer allows it.
func (d *SimpleDebouncer) Debounce(fn func()) bool {
	if d.count.Add(1) > 2 {
		d.count.Add(-1)
		return false
	}
	d.m.Lock()
	fn()
	d.count.Add(-1)
	d.m.Unlock()
	return true
}
