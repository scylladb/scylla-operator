package v1

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
)

var (
	prometheusRulesFileRegex = regexp.MustCompile(`^[^/]+\.yml$`)
)

type AccessChecker[T any] interface {
	Get() T
	Accessed() bool
}

type accessChecker[T any] struct {
	value    T
	accessed bool
}

func NewAccessChecker[T any](value T) AccessChecker[T] {
	return &accessChecker[T]{
		value:    value,
		accessed: false,
	}
}

func (ac *accessChecker[T]) Get() T {
	ac.accessed = true
	return ac.value
}

func (ac *accessChecker[T]) Accessed() bool {
	return ac.accessed
}

type fileAccessChecker = AccessChecker[string]

type PrometheusRulesMap map[string]fileAccessChecker

func NewPrometheusRulesFromFS(filesystem embed.FS) (PrometheusRulesMap, error) {
	prs := make(PrometheusRulesMap)

	root := "."
	err := fs.WalkDir(filesystem, root, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		if !prometheusRulesFileRegex.MatchString(d.Name()) {
			return nil
		}

		fullPath := filepath.Join(root, path)
		data, err := fs.ReadFile(filesystem, fullPath)
		if err != nil {
			return fmt.Errorf("can't read file %s: %w", fullPath, err)
		}

		shortName := d.Name()
		prs[shortName] = NewAccessChecker(string(data))

		return nil
	})
	if err != nil {
		return prs, fmt.Errorf("can't read prometheus rules from top level dir: %w", err)
	}

	return prs, nil
}
