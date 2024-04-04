package object

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

// LazyObject defers underlying object creation.
type LazyObject struct {
	ctx    context.Context
	f      fs.Fs
	remote string
	once   sync.Once

	o   fs.Object
	err error
}

// NewLazyObject returns an object that defers the real object creation
// until the object is needed.
func NewLazyObject(ctx context.Context, f fs.Fs, remote string) *LazyObject {
	return &LazyObject{ctx: ctx, f: f, remote: remote}
}

var _ fs.Object = (*LazyObject)(nil)

func (l *LazyObject) init() {
	l.once.Do(func() {
		fs.Debugf(l.f, "lazy init %s", l.remote)
		l.o, l.err = l.f.NewObject(l.ctx, l.remote)
		if l.err != nil {
			fs.Errorf(l.f, "lazy init %s failed: %s", l.remote, l.err)
		}
	})
}

// String returns a description of the Object
func (l *LazyObject) String() string {
	return l.remote
}

// Remote returns the remote path
func (l *LazyObject) Remote() string {
	return l.remote
}

// ModTime returns the modification date of the file
func (l *LazyObject) ModTime(ctx context.Context) time.Time {
	l.init()

	if l.err != nil {
		return time.Time{}
	}
	return l.o.ModTime(ctx)
}

// Size returns the size of the file
func (l *LazyObject) Size() int64 {
	l.init()

	if l.err != nil {
		return 0
	}
	return l.o.Size()
}

// Fs returns read only access to the Fs that this object is part of
func (l *LazyObject) Fs() fs.Info {
	return l.f
}

// Hash returns the requested hash of the contents
func (l *LazyObject) Hash(ctx context.Context, ty hash.Type) (string, error) {
	l.init()

	if l.err != nil {
		return "", l.err
	}
	return l.o.Hash(ctx, ty)
}

// Storable says whether this object can be stored
func (l *LazyObject) Storable() bool {
	l.init()

	if l.err != nil {
		return false
	}
	return l.o.Storable()
}

// SetModTime sets the metadata on the object to set the modification date
func (l *LazyObject) SetModTime(ctx context.Context, t time.Time) error {
	l.init()

	if l.err != nil {
		return l.err
	}
	return l.o.SetModTime(ctx, t)
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (l *LazyObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	l.init()

	if l.err != nil {
		return nil, l.err
	}
	return l.o.Open(ctx, options...)
}

// Update in to the object with the modTime given of the given size
func (l *LazyObject) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	l.init()

	if l.err != nil {
		return l.err
	}
	return l.o.Update(ctx, in, src, options...)
}

// Remove this object
func (l *LazyObject) Remove(ctx context.Context) error {
	l.init()

	if l.err != nil {
		return l.err
	}
	return l.o.Remove(ctx)
}
