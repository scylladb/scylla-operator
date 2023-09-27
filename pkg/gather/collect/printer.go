package collect

import (
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

type SuffixInterface interface {
	GetSuffix() string
}

type ResourcePrinterInterface interface {
	PrintObj(*ResourceInfo, runtime.Object, io.Writer) error
	SuffixInterface
}

type YAMLPrinter struct {
}

var _ ResourcePrinterInterface = &YAMLPrinter{}

func (p *YAMLPrinter) PrintObj(resourceInfo *ResourceInfo, obj runtime.Object, w io.Writer) error {
	output, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(w, string(output))
	if err != nil {
		return err
	}

	return nil
}
func (p *YAMLPrinter) GetSuffix() string {
	return ".yaml"
}

type OmitManagedFieldsPrinter struct {
	Delegate ResourcePrinterInterface
}

var _ ResourcePrinterInterface = &OmitManagedFieldsPrinter{}

func (p *OmitManagedFieldsPrinter) PrintObj(resourceInfo *ResourceInfo, obj runtime.Object, w io.Writer) error {
	if obj == nil {
		return p.Delegate.PrintObj(resourceInfo, obj, w)
	}

	a, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	a.SetManagedFields(nil)

	return p.Delegate.PrintObj(resourceInfo, obj, w)
}

func (p *OmitManagedFieldsPrinter) GetSuffix() string {
	return p.Delegate.GetSuffix()
}
