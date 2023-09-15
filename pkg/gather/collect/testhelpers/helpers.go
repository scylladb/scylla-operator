package testhelpers

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakediscovery "k8s.io/client-go/discovery/fake"
)

// FakeDiscoveryWithSPR overrides ServerPreferredResources because client-go always returns (nil, nil).
type FakeDiscoveryWithSPR struct {
	*fakediscovery.FakeDiscovery
}

func (d *FakeDiscoveryWithSPR) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	// We don't use multiple versions in tests so this just needs to return a list.
	_, l, err := d.ServerGroupsAndResources()
	return l, err
}

type File struct {
	Name    string
	Content string
}

type GatherDump struct {
	EmptyDirs []string
	Files     []File
}

func (gd *GatherDump) Sort() {
	sort.SliceStable(gd.EmptyDirs, func(i, j int) bool {
		return gd.EmptyDirs[i] < gd.EmptyDirs[j]
	})
	sort.SliceStable(gd.Files, func(i, j int) bool {
		return gd.Files[i].Name < gd.Files[j].Name
	})
}

func IsDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	} else {
		return false, err
	}
}

func ReadGatherDump(gatherPath string) (*GatherDump, error) {
	gd := &GatherDump{}

	err := filepath.WalkDir(gatherPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if p == gatherPath {
			return nil
		}

		relativePath, err := filepath.Rel(gatherPath, p)
		if err != nil {
			return fmt.Errorf("can't make relative path from file %q to %q: %w", p, gatherPath, err)
		}

		if d.IsDir() {
			empty, err := IsDirEmpty(p)
			if err != nil {
				return fmt.Errorf("can't determine whether dir %q is empty: %w", p, err)
			}
			// We only care about empty dirs, others will be accounted for by the files or empty dirs inside them.
			if empty {
				gd.EmptyDirs = append(gd.EmptyDirs, relativePath)
			}
		} else {
			data, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("can't read file %q: %w", p, err)
			}
			gd.Files = append(gd.Files, File{
				Name:    relativePath,
				Content: string(data),
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("can't read gather dir %q: %w", gatherPath, err)
	}

	gd.Sort()

	return gd, nil
}
