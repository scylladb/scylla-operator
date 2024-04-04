// Copyright (C) 2017 ScyllaDB

package rcserver

import (
	"context"
	"path"
	"path/filepath"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/rc"
)

type paramsValidator func(ctx context.Context, in rc.Params) error

func wrap(fn rc.Func, v paramsValidator) rc.Func {
	return func(ctx context.Context, in rc.Params) (rc.Params, error) {
		if err := v(ctx, in); err != nil {
			return nil, err
		}
		return fn(ctx, in)
	}
}

// pathHasPrefix reads "fs" and "remote" params, evaluates absolute path and
// ensures it has the required prefix.
func pathHasPrefix(prefix string) paramsValidator {
	return func(ctx context.Context, in rc.Params) error {
		_, p, err := joined(in, "fs", "remote")
		if err != nil {
			return err
		}

		// Strip bucket name
		i := strings.Index(p, "/")
		p = p[i+1:]

		if !strings.HasPrefix(p, prefix) {
			return fs.ErrorPermissionDenied
		}
		return nil
	}
}

func localToRemote() paramsValidator {
	return func(ctx context.Context, in rc.Params) error {
		fsrc, err := rc.GetFsNamed(ctx, in, "srcFs")
		if err != nil {
			return err
		}
		if !fsrc.Features().IsLocal {
			return fs.ErrorPermissionDenied
		}
		fdst, err := rc.GetFsNamed(ctx, in, "dstFs")
		if err != nil {
			return err
		}
		if fdst.Features().IsLocal {
			return fs.ErrorPermissionDenied
		}
		return nil
	}
}

func remoteToLocal() paramsValidator {
	return func(ctx context.Context, in rc.Params) error {
		fsrc, err := rc.GetFsNamed(ctx, in, "srcFs")
		if err != nil {
			return err
		}
		if fsrc.Features().IsLocal {
			return fs.ErrorPermissionDenied
		}
		fdst, err := rc.GetFsNamed(ctx, in, "dstFs")
		if err != nil {
			return err
		}
		if !fdst.Features().IsLocal {
			return fs.ErrorPermissionDenied
		}
		return nil
	}
}

func sameDir() paramsValidator {
	return func(ctx context.Context, in rc.Params) error {
		srcName, srcPath, err := joined(in, "srcFs", "srcRemote")
		if err != nil {
			return err
		}
		dstName, dstPath, err := joined(in, "dstFs", "dstRemote")
		if err != nil {
			return err
		}
		if srcName != dstName || path.Dir(srcPath) != path.Dir(dstPath) {
			return fs.ErrorPermissionDenied
		}
		return nil
	}
}

func joined(in rc.Params, fsName, remoteName string) (configName, remotePath string, err error) {
	f, err := in.GetString(fsName)
	if err != nil {
		return "", "", err
	}
	remote, err := in.GetString(remoteName)
	if err != nil {
		return "", "", err
	}
	return join(f, remote)
}

func join(f, remote string) (configName, remotePath string, err error) {
	configName, fsPath, err := fspath.Parse(f)
	if err != nil {
		return "", "", err
	}
	return configName, filepath.Clean(path.Join(fsPath, remote)), nil
}
