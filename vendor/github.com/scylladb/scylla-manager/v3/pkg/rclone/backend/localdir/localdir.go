// Copyright (C) 2017 ScyllaDB

// Package localdir is rclone backend based on local backend provided by rclone.
// The difference from local is that data is always rooted at a directory
// that can be specified dynamically on creation.
package localdir

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
)

const (
	linkSuffix = ".rclonelink"
)

// Init registers new data provider with rclone.
func Init(name, description, rootDir string) {
	fsi := &fs.RegInfo{
		Name:        name,
		Description: description,
		NewFs:       NewFs(rootDir),
		Options: []fs.Option{{
			Name: "nounc",
			Help: "Disable UNC (long path names) conversion on Windows",
			Examples: []fs.OptionExample{{
				Value: "true",
				Help:  "Disables long file names",
			}},
		}, {
			Name:     "copy_links",
			Help:     "Follow symlinks and copy the pointed to item.",
			Default:  false,
			NoPrefix: true,
			ShortOpt: "L",
			Advanced: true,
		}, {
			Name:     "links",
			Help:     "Translate symlinks to/from regular files with a '" + linkSuffix + "' extension",
			Default:  false,
			NoPrefix: true,
			ShortOpt: "l",
			Advanced: true,
		}, {
			Name: "skip_links",
			Help: `Don't warn about skipped symlinks.
This flag disables warning messages on skipped symlinks or junction
points, as you explicitly acknowledge that they should be skipped.`,
			Default:  false,
			NoPrefix: true,
			Advanced: true,
		}, {
			Name: "no_unicode_normalization",
			Help: `Don't apply unicode normalization to paths and filenames (Deprecated)

This flag is deprecated now.  Rclone no longer normalizes unicode file
names, but it compares them with unicode normalization in the sync
routine instead.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "no_check_updated",
			Help: `Don't check to see if the files change during upload

Normally rclone checks the size and modification time of files as they
are being uploaded and aborts with a message which starts "can't copy
- source file is being updated" if the file changes during upload.

However on some file systems this modification time check may fail (eg
[Glusterfs #2206](https://github.com/rclone/rclone/issues/2206)) so this
check can be disabled with this flag.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:     "one_file_system",
			Help:     "Don't cross filesystem boundaries (unix/macOS only).",
			Default:  false,
			NoPrefix: true,
			ShortOpt: "x",
			Advanced: true,
		}, {
			Name: "case_sensitive",
			Help: `Force the filesystem to report itself as case sensitive.

Normally the local backend declares itself as case insensitive on
Windows/macOS and case sensitive for everything else.  Use this flag
to override the default choice.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "case_insensitive",
			Help: `Force the filesystem to report itself as case insensitive

Normally the local backend declares itself as case insensitive on
Windows/macOS and case sensitive for everything else.  Use this flag
to override the default choice.`,
			Default:  false,
			Advanced: true,
		}},
	}

	fs.Register(fsi)
}

func NewFs(rootDir string) func(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	return func(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
		// filepath.Clean will turn everything that goes up and beyond root into
		// a single /.
		// We are prepending slash to turn input into an absolute path.
		p := filepath.Clean("/" + root)
		if len(root) > 1 && p == "/" {
			// If root has more than one byte and after cleanPath we end up with
			// empty path then we received invalid input.
			return nil, errors.Wrap(fs.ErrorObjectNotFound, "accessing path outside of root")
		}
		var path string
		if strings.HasPrefix(p, rootDir) {
			path = p
		} else {
			path = filepath.Join(rootDir, p)
		}
		return local.NewFs(ctx, name, path, m)
	}
}
