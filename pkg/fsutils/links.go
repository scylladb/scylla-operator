package fsutils

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveSymlinks resolves any symlinks in the path for any subpath that exists.
func ResolveSymlinks(path string) (string, error) {
	_, statErr := os.Stat(path)
	switch {
	case statErr == nil:
		return filepath.EvalSymlinks(path)

	case os.IsNotExist(statErr):
		resolvedParentPath, resolveErr := ResolveSymlinks(filepath.Dir(path))
		if resolveErr != nil {
			return "", resolveErr
		}

		return filepath.Join(resolvedParentPath, filepath.Base(path)), nil

	default:
		return "", fmt.Errorf("can't stat mount point %q: %w", path, statErr)
	}
}
