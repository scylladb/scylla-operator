package collect

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/klog/v2"
)

type ResourceWriter struct {
	baseDir  string
	printers []ResourcePrinterInterface
}

func NewResourceWriter(baseDir string, printers []ResourcePrinterInterface) *ResourceWriter {
	return &ResourceWriter{
		baseDir:  baseDir,
		printers: printers,
	}
}

func (c *ResourceWriter) GetResourceDir(obj kubeinterfaces.ObjectInterface, resourceInfo *ResourceInfo) (string, error) {
	scope := resourceInfo.Scope.Name()
	switch scope {
	case meta.RESTScopeNameNamespace:
		return filepath.Join(
			c.baseDir,
			namespacesDirName,
			obj.GetNamespace(),
			resourceInfo.Resource.GroupResource().String(),
		), nil

	case meta.RESTScopeNameRoot:
		return filepath.Join(
			c.baseDir,
			clusterScopedDirName,
			resourceInfo.Resource.GroupResource().String(),
		), nil

	default:
		return "", fmt.Errorf("unrecognized scope %q", scope)
	}
}

func (c *ResourceWriter) WriteObject(ctx context.Context, dirPath string, obj kubeinterfaces.ObjectInterface, resourceInfo *ResourceInfo) error {
	var err error
	for _, printer := range c.printers {
		filePath := filepath.Join(dirPath, obj.GetName()+printer.GetSuffix())
		err = writeObject(printer, filePath, resourceInfo, obj)
		if err != nil {
			return fmt.Errorf("can't write object: %w", err)
		}
	}

	return nil
}

func (c *ResourceWriter) WriteResource(ctx context.Context, obj kubeinterfaces.ObjectInterface, resourceInfo *ResourceInfo) error {
	resourceDir, err := c.GetResourceDir(obj, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't get resourceDir: %q", err)
	}

	err = os.MkdirAll(resourceDir, 0770)
	if err != nil {
		return fmt.Errorf("can't create resource dir %q: %w", resourceDir, err)
	}

	err = c.WriteObject(ctx, resourceDir, obj, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't write object: %w", err)
	}

	return nil
}

func writeObject(printer ResourcePrinterInterface, filePath string, resourceInfo *ResourceInfo, obj kubeinterfaces.ObjectInterface) error {
	buf := bytes.NewBuffer(nil)
	err := printer.PrintObj(resourceInfo, obj, buf)
	if err != nil {
		return fmt.Errorf("can't print object %q (%s): %w", naming.ObjRef(obj), resourceInfo.Resource, err)
	}

	err = os.WriteFile(filePath, buf.Bytes(), 0770)
	if err != nil {
		return fmt.Errorf("can't write file %q: %w", filePath, err)
	}

	klog.V(4).InfoS("Written resource", "Path", filePath)

	return nil
}
