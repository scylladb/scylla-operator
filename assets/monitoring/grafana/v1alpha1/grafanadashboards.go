package v1alpha1

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
)

var (
	grafanaDashboardsFileRegex = regexp.MustCompile(`^[^/]+\.json$`)

	// FIXME: remove the exclusions and fix https://github.com/scylladb/scylla-operator/issues/2822
	// before supporting ScyllaDB 2025.3.
	platformDashboardsToExclude = []string{
		"scylladb-2025.3.0",
		"scylladb-2025.3",
	}
)

func gzipMapData(uncompressedMap map[string]string) (map[string]string, error) {
	res := make(map[string]string, len(uncompressedMap))
	for k, v := range uncompressedMap {
		var buf bytes.Buffer
		b64Writer := base64.NewEncoder(base64.StdEncoding, &buf)

		gzw, err := gzip.NewWriterLevel(b64Writer, gzip.BestCompression)
		if err != nil {
			return nil, fmt.Errorf("can't create gzip writer: %w", err)
		}

		_, err = gzw.Write([]byte(v))
		if err != nil {
			return nil, fmt.Errorf("can't write value of key %q into gzip writer: %w", k, err)
		}

		err = gzw.Close()
		if err != nil {
			return nil, fmt.Errorf("can't close gzip writer for key %q: %w", k, err)
		}

		err = b64Writer.Close()
		if err != nil {
			return nil, fmt.Errorf("can't close base64 writer for key %q: %w", k, err)
		}

		res[fmt.Sprintf("%s.gz.base64", k)] = buf.String()
	}

	return res, nil
}

type GrafanaDashboardFolder map[string]string

type GrafanaDashboardsFoldersMap map[string]GrafanaDashboardFolder

func NewGrafanaDashboardsFromFS(filesystem embed.FS, root string) (GrafanaDashboardsFoldersMap, error) {
	topEntries, err := fs.ReadDir(filesystem, root)
	if err != nil {
		return nil, fmt.Errorf("can't read top level directory: %w", err)
	}

	grafanaDashboardsFoldersMap := GrafanaDashboardsFoldersMap{}
	for _, e := range topEntries {
		if !e.IsDir() {
			continue
		}

		// Exclude platform dashboards that are known to cause https://github.com/scylladb/scylla-operator/issues/2822.
		// TODO: get rid of this exclusion once the root cause is fixed.
		directoryName := filepath.Base(e.Name())
		if slices.Contains(platformDashboardsToExclude, directoryName) {
			continue
		}

		var versionEntries []fs.DirEntry
		p := filepath.Join(root, e.Name())
		versionEntries, err = fs.ReadDir(filesystem, p)
		if err != nil {
			return nil, fmt.Errorf("can't read directory %q: %w", p, err)
		}

		grafanaDashboardFolder := GrafanaDashboardFolder{}
		for _, ve := range versionEntries {
			fullPath := filepath.Join(p, ve.Name())

			if ve.IsDir() {
				return nil, fmt.Errorf("unexpected folder %q", fullPath)
			}

			if !grafanaDashboardsFileRegex.MatchString(ve.Name()) {
				continue
			}

			var content []byte
			content, err = fs.ReadFile(filesystem, fullPath)
			if err != nil {
				return nil, fmt.Errorf("can't read file %q: %w", fullPath, err)
			}
			grafanaDashboardFolder[ve.Name()] = string(content)
		}

		var compressedFolder map[string]string
		compressedFolder, err = gzipMapData(grafanaDashboardFolder)
		if err != nil {
			return nil, fmt.Errorf("can't compress grafana folder %q: %w", e.Name(), err)
		}
		grafanaDashboardsFoldersMap[e.Name()] = compressedFolder
	}

	return grafanaDashboardsFoldersMap, nil
}
