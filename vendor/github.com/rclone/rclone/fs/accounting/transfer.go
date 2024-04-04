package accounting

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
)

// AggregatedTransferInfo aggregated transfer statistics.
type AggregatedTransferInfo struct {
	Uploaded    int64     `json:"uploaded"`
	Skipped     int64     `json:"skipped"`
	Failed      int64     `json:"failed"`
	Size        int64     `json:"size"`
	Error       error     `json:"error"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

func (ai *AggregatedTransferInfo) update(t *Transfer) {
	if t.checking {
		return
	}

	if ai.StartedAt.IsZero() {
		ai.StartedAt = t.startedAt
	}
	if !t.startedAt.IsZero() && t.startedAt.Before(ai.StartedAt) {
		ai.StartedAt = t.startedAt
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.completedAt.IsZero() && ai.CompletedAt.Before(t.completedAt) {
		ai.CompletedAt = t.completedAt
	}

	var b, s int64 = 0, t.size
	if t.acc != nil {
		b, s = t.acc.progress()
	}
	ai.Size += s
	if t.err != nil {
		ai.Failed += s
		ai.Error = errors.Join(ai.Error, t.err)
	} else {
		ai.Uploaded += b
	}
}

func (ai *AggregatedTransferInfo) merge(other AggregatedTransferInfo) {
	ai.Uploaded += other.Uploaded
	ai.Skipped += other.Skipped
	ai.Failed += other.Failed
	ai.Size += other.Size
	ai.Error = errors.Join(ai.Error, other.Error)
	if !other.StartedAt.IsZero() && other.StartedAt.Before(ai.StartedAt) {
		ai.StartedAt = other.StartedAt
	}
	if ai.CompletedAt.Before(other.CompletedAt) {
		ai.CompletedAt = other.CompletedAt
	}
}

// TransferSnapshot represents state of an account at point in time.
type TransferSnapshot struct {
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	Bytes       int64     `json:"bytes"`
	Checked     bool      `json:"checked"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Error       error     `json:"-"`
	Group       string    `json:"group"`
}

// MarshalJSON implements json.Marshaler interface.
func (as TransferSnapshot) MarshalJSON() ([]byte, error) {
	err := ""
	if as.Error != nil {
		err = as.Error.Error()
	}

	type Alias TransferSnapshot
	return json.Marshal(&struct {
		Error string `json:"error"`
		Alias
	}{
		Error: err,
		Alias: (Alias)(as),
	})
}

// Transfer keeps track of initiated transfers and provides access to
// accounting functions.
// Transfer needs to be closed on completion.
type Transfer struct {
	// these are initialised at creation and may be accessed without locking
	stats     *StatsInfo
	remote    string
	size      int64
	startedAt time.Time
	checking  bool

	// Protects all below
	//
	// NB to avoid deadlocks we must release this lock before
	// calling any methods on Transfer.stats.  This is because
	// StatsInfo calls back into Transfer.
	mu          sync.RWMutex
	acc         *Account
	err         error
	completedAt time.Time
}

// newCheckingTransfer instantiates new checking of the object.
func newCheckingTransfer(stats *StatsInfo, obj fs.Object) *Transfer {
	return newTransferRemoteSize(stats, obj.Remote(), obj.Size(), true)
}

// newTransfer instantiates new transfer.
func newTransfer(stats *StatsInfo, obj fs.Object) *Transfer {
	return newTransferRemoteSize(stats, obj.Remote(), obj.Size(), false)
}

func newTransferRemoteSize(stats *StatsInfo, remote string, size int64, checking bool) *Transfer {
	tr := &Transfer{
		stats:     stats,
		remote:    remote,
		size:      size,
		startedAt: time.Now(),
		checking:  checking,
	}
	stats.AddTransfer(tr)
	return tr
}

// Done ends the transfer.
// Must be called after transfer is finished to run proper cleanups.
func (tr *Transfer) Done(ctx context.Context, err error) {
	if err != nil {
		err = tr.stats.Error(err)

		tr.mu.Lock()
		tr.err = err
		tr.mu.Unlock()
	}

	tr.mu.RLock()
	acc := tr.acc
	tr.mu.RUnlock()

	ci := fs.GetConfig(ctx)
	if acc != nil {
		// Close the file if it is still open
		if err := acc.Close(); err != nil {
			fs.LogLevelPrintf(ci.StatsLogLevel, nil, "can't close account: %+v\n", err)
		}
		// Signal done with accounting
		acc.Done()
		// free the account since we may keep the transfer
		acc = nil
	}

	tr.mu.Lock()
	tr.completedAt = time.Now()
	tr.mu.Unlock()

	if tr.checking {
		tr.stats.DoneChecking(tr.remote)
	} else {
		tr.stats.DoneTransferring(tr.remote, err == nil)
	}
	tr.stats.PruneTransfers()
}

// Reset allows to switch the Account to another transfer method.
func (tr *Transfer) Reset(ctx context.Context) {
	tr.mu.RLock()
	acc := tr.acc
	tr.acc = nil
	tr.mu.RUnlock()
	ci := fs.GetConfig(ctx)

	if acc != nil {
		if err := acc.Close(); err != nil {
			fs.LogLevelPrintf(ci.StatsLogLevel, nil, "can't close account: %+v\n", err)
		}
	}
}

// Account returns reader that knows how to keep track of transfer progress.
func (tr *Transfer) Account(ctx context.Context, in io.ReadCloser) *Account {
	tr.mu.Lock()
	if tr.acc == nil {
		tr.acc = newAccountSizeName(ctx, tr.stats, in, tr.size, tr.remote)
	} else {
		tr.acc.UpdateReader(ctx, in)
	}
	tr.mu.Unlock()
	return tr.acc
}

// TimeRange returns the time transfer started and ended at. If not completed
// it will return zero time for end time.
func (tr *Transfer) TimeRange() (time.Time, time.Time) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return tr.startedAt, tr.completedAt
}

// IsDone returns true if transfer is completed.
func (tr *Transfer) IsDone() bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return !tr.completedAt.IsZero()
}

// Snapshot produces stats for this account at point in time.
func (tr *Transfer) Snapshot() TransferSnapshot {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	var s, b int64 = tr.size, 0
	if tr.acc != nil {
		b, s = tr.acc.progress()
	}
	return TransferSnapshot{
		Name:        tr.remote,
		Checked:     tr.checking,
		Size:        s,
		Bytes:       b,
		StartedAt:   tr.startedAt,
		CompletedAt: tr.completedAt,
		Error:       tr.err,
		Group:       tr.stats.group,
	}
}

// rcStats returns stats for the transfer suitable for the rc
func (tr *Transfer) rcStats() rc.Params {
	return rc.Params{
		"name": tr.remote, // no locking needed to access thess
		"size": tr.size,
	}
}
