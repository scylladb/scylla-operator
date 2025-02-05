package gocql

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Tracer is the interface implemented by query tracers. Tracers have the
// ability to obtain a detailed event log of all events that happened during
// the execution of a query from Cassandra. Gathering this information might
// be essential for debugging and optimizing queries, but this feature should
// not be used on production systems with very high load.
type Tracer interface {
	Trace(traceId []byte)
}

type TraceWriter struct {
	session *Session
	w       io.Writer
	mu      sync.Mutex

	maxAttempts   int
	sleepInterval time.Duration
}

// NewTraceWriter returns a simple Tracer implementation that outputs
// the event log in a textual format.
func NewTraceWriter(session *Session, w io.Writer) *TraceWriter {
	return &TraceWriter{session: session, w: w, maxAttempts: 5, sleepInterval: 3 * time.Millisecond}
}

func (t *TraceWriter) SetMaxAttempts(maxAttempts int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.maxAttempts = maxAttempts
}

func (t *TraceWriter) SetSleepInterval(sleepInterval time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.sleepInterval = sleepInterval
}

func (t *TraceWriter) Trace(traceId []byte) {
	var (
		timestamp time.Time
		activity  string
		source    string
		elapsed   int
		thread    string
	)

	t.mu.Lock()
	defer t.mu.Unlock()

	fetchAttempts := 1
	if t.maxAttempts > 0 {
		fetchAttempts = t.maxAttempts
	}

	isDone := false
	for i := 0; i < fetchAttempts; i++ {
		var duration int

		iter := t.session.control.query(`SELECT duration
			FROM system_traces.sessions
			WHERE session_id = ?`, traceId)
		iter.Scan(&duration)
		if duration > 0 {
			isDone = true
		}

		if err := iter.Close(); err != nil {
			fmt.Fprintln(t.w, "Error:", err)
			return
		}

		if isDone || i == fetchAttempts-1 {
			break
		}

		time.Sleep(t.sleepInterval)
	}
	if !isDone {
		fmt.Fprintln(t.w, "Error: failed to wait tracing to complete. !!! Tracing is incomplete !!!")
	}

	var (
		coordinator string
		duration    int
	)

	iter := t.session.control.query(`SELECT coordinator, duration
		FROM system_traces.sessions
		WHERE session_id = ?`, traceId)

	iter.Scan(&coordinator, &duration)
	if err := iter.Close(); err != nil {
		fmt.Fprintln(t.w, "Error:", err)
		return
	}

	fmt.Fprintf(t.w, "Tracing session %016x (coordinator: %s, duration: %v):\n",
		traceId, coordinator, time.Duration(duration)*time.Microsecond)

	iter = t.session.control.query(`SELECT event_id, activity, source, source_elapsed, thread
			FROM system_traces.events
			WHERE session_id = ?`, traceId)

	for iter.Scan(&timestamp, &activity, &source, &elapsed, &thread) {
		fmt.Fprintf(t.w, "%s: %s [%s] (source: %s, elapsed: %d)\n",
			timestamp.Format("2006/01/02 15:04:05.999999"), activity, thread, source, elapsed)
	}

	if err := iter.Close(); err != nil {
		fmt.Fprintln(t.w, "Error:", err)
	}
}

type TracerEnhanced struct {
	session  *Session
	traceIDs [][]byte
	mu       sync.Mutex
}

func NewTracer(session *Session) *TracerEnhanced {
	return &TracerEnhanced{session: session}
}

func (t *TracerEnhanced) Trace(traceId []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.traceIDs = append(t.traceIDs, traceId)
}

func (t *TracerEnhanced) AllTraceIDs() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.traceIDs
}

func (t *TracerEnhanced) IsReady(traceId []byte) (bool, error) {
	isDone := false
	var duration int

	iter := t.session.control.query(`SELECT duration
		FROM system_traces.sessions
		WHERE session_id = ?`, traceId)
	iter.Scan(&duration)
	if duration > 0 {
		isDone = true
	}

	if err := iter.Close(); err != nil {
		return false, err
	}

	if isDone {
		return true, nil
	}

	return false, nil
}

func (t *TracerEnhanced) GetCoordinatorTime(traceId []byte) (string, time.Duration, error) {
	var (
		coordinator string
		duration    int
	)

	iter := t.session.control.query(`SELECT coordinator, duration
		FROM system_traces.sessions
		WHERE session_id = ?`, traceId)

	iter.Scan(&coordinator, &duration)
	if err := iter.Close(); err != nil {
		return coordinator, time.Duration(duration) * time.Microsecond, err
	}

	return coordinator, time.Duration(duration) * time.Microsecond, nil
}

type TraceEntry struct {
	Timestamp time.Time
	Activity  string
	Source    string
	Elapsed   int
	Thread    string
}

func (t *TracerEnhanced) GetActivities(traceId []byte) ([]TraceEntry, error) {
	iter := t.session.control.query(`SELECT event_id, activity, source, source_elapsed, thread
		FROM system_traces.events
		WHERE session_id = ?`, traceId)

	var (
		timestamp time.Time
		activity  string
		source    string
		elapsed   int
		thread    string
	)

	var activities []TraceEntry

	for iter.Scan(&timestamp, &activity, &source, &elapsed, &thread) {
		activities = append(activities, TraceEntry{Timestamp: timestamp, Activity: activity, Source: source, Elapsed: elapsed, Thread: thread})
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return activities, nil
}
