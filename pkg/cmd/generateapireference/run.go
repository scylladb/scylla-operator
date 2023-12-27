package generateapireference

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/scylladb/scylla-operator/pkg/assets"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/signals"
	programversion "github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

const (
	groupFileNameSuffix     = ".rst"
	gvIndexTemplateFileName = "group.rst.tmpl"
	kindFileNameSuffix      = ".rst"
	kindTemplateFileName    = "kind.rst.tmpl"
)

type ResourceInfo struct {
	APIVersion       string
	Group            string
	Version          string
	Names            apiextensionsv1.CustomResourceDefinitionNames
	Scope            apiextensionsv1.ResourceScope
	Served           bool
	Storage          bool
	Property         apiextensionsv1.JSONSchemaProps
	NestedProperties map[string]apiextensionsv1.JSONSchemaProps
}

func indexNestedItems(arrayProps apiextensionsv1.JSONSchemaProps, key string, accumulator *map[string]apiextensionsv1.JSONSchemaProps) {
	switch arrayProps.Type {
	case "object":
		(*accumulator)[key] = arrayProps
		indexNestedProps(arrayProps.Properties, key, accumulator)
	case "array":
		indexNestedItems(*arrayProps.Items.Schema, key, accumulator)
	default:
		break
	}
}

func indexNestedProps(props map[string]apiextensionsv1.JSONSchemaProps, propsKey string, accumulator *map[string]apiextensionsv1.JSONSchemaProps) {
	for k, v := range props {
		var key string
		switch v.Type {
		case "array":
			key = fmt.Sprintf("%s.%s[]", propsKey, k)
		case "object":
			fallthrough
		default:
			key = fmt.Sprintf("%s.%s", propsKey, k)
		}

		switch v.Type {
		case "object":
			(*accumulator)[key] = v
			indexNestedProps(v.Properties, key, accumulator)
		case "array":
			indexNestedItems(*v.Items.Schema, key, accumulator)
		default:
		}
	}
}

// IndexNestedProps will traverse all object and for any object or array that's embedded,
// it will create a key value pair in the map that's returned.
// It keeps the nesting in place so the is still a context to e.g. determine type (array of stings).
func IndexNestedProps(props map[string]apiextensionsv1.JSONSchemaProps) map[string]apiextensionsv1.JSONSchemaProps {
	res := map[string]apiextensionsv1.JSONSchemaProps{}
	indexNestedProps(props, "", &res)
	return res
}

func getLabelForKey(key string) string {
	key, _ = strings.CutPrefix(key, ".")
	return strings.Replace(key, ".", "-", -1)
}

func ensurePrefix(prefix string, s string) string {
	hasPrefix := strings.HasPrefix(s, prefix)
	if hasPrefix {
		return s
	}
	return prefix + s
}

func getObjectLink(key string, fieldProps apiextensionsv1.JSONSchemaProps) string {
	switch fieldProps.Type {
	case "object":
		return "." + key
	case "array":
		l := getObjectLink(key, *fieldProps.Items.Schema)
		if len(l) == 0 {
			return l
		}
		return l + "[]"
	default:
		return ""
	}
}

var templateFuncs = template.FuncMap{
	"indentNext":   assets.IndentNext,
	"repeat":       assets.Repeat,
	"map":          assets.MakeMap,
	"labelForKey":  getLabelForKey,
	"ensurePrefix": ensurePrefix,
	"objectLink":   getObjectLink,
}

func (o *GenerateAPIRefsOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	klog.V(1).Infof("%q version %q", cmd.CommandPath(), programversion.Get())
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	return o.run(ctx, streams)
}

func (o *GenerateAPIRefsOptions) parseTemplate(tmplFileName string) (*template.Template, error) {
	tmplFile := filepath.Join(o.TemplatesDir, tmplFileName)
	t, err := template.New(tmplFileName).Funcs(templateFuncs).ParseFiles(tmplFile)
	if err != nil {
		return nil, fmt.Errorf("can't parse template file %q: %w", tmplFile, err)
	}

	return t, nil
}

func (o *GenerateAPIRefsOptions) run(ctx context.Context, streams genericclioptions.IOStreams) error {
	err := os.Mkdir(o.OutputDir, 0777)
	if err == nil {
		klog.V(2).InfoS("Created output directory", "Path", o.OutputDir)
	} else if os.IsExist(err) {
		klog.V(2).InfoS("Using existing output directory", "Path", o.OutputDir)
	} else {
		return fmt.Errorf("can't create output directory %q: %w", o.OutputDir, err)
	}

	klog.V(1).InfoS("Parsing CRD files")

	groups := map[string][]*ResourceInfo{}
	for _, crdPath := range o.CustomResourceDefinitionPaths {
		crdBytes, err := os.ReadFile(crdPath)
		if err != nil {
			return fmt.Errorf("can't read crd file %q: %w", crdPath, err)
		}

		crd := &apiextensionsv1.CustomResourceDefinition{}
		err = runtime.DecodeInto(
			Codecs.DecoderToVersion(Codecs.UniversalDeserializer(), apiextensionsv1.SchemeGroupVersion),
			crdBytes,
			crd,
		)
		if err != nil {
			return fmt.Errorf("can't decode crd file %q: %w", crdPath, err)
		}

		for _, version := range crd.Spec.Versions {
			if version.Schema.OpenAPIV3Schema == nil {
				return fmt.Errorf("OpenAPIV3Schema is missing")
			}

			if groups[crd.Spec.Group] == nil {
				groups[crd.Spec.Group] = []*ResourceInfo{}
			}
			gv := metav1.GroupVersion{Group: crd.Spec.Group, Version: version.Name}
			groups[crd.Spec.Group] = append(groups[crd.Spec.Group], &ResourceInfo{
				APIVersion:       gv.String(),
				Group:            gv.Group,
				Version:          gv.Version,
				Names:            crd.Spec.Names,
				Scope:            crd.Spec.Scope,
				Served:           version.Served,
				Storage:          version.Storage,
				Property:         *version.Schema.OpenAPIV3Schema,
				NestedProperties: IndexNestedProps(version.Schema.OpenAPIV3Schema.Properties),
			})
		}
	}

	if len(groups) == 0 {
		return fmt.Errorf("no API group found in CRD files")
	}

	klog.V(1).InfoS("Parsing templates", "Directory", o.TemplatesDir)

	gvIndexTemplate, err := o.parseTemplate(gvIndexTemplateFileName)
	if err != nil {
		return err
	}

	kindTemplate, err := o.parseTemplate(kindTemplateFileName)
	if err != nil {
		return err
	}

	klog.V(1).InfoS("Generating templates")

	for group, resourceInfos := range groups {
		groupDir := filepath.Join(o.OutputDir, group)
		err := os.Mkdir(groupDir, 0777)
		if err == nil {
			klog.V(2).InfoS("Created group directory", "Path", groupDir)
		} else if os.IsExist(err) {
			klog.V(2).InfoS("Using existing group directory", "Path", groupDir)
		} else {
			return fmt.Errorf("can't create group directory %q: %w", groupDir, err)
		}

		data, err := assets.RenderTemplate(gvIndexTemplate, map[string]string{"Group": group})
		if err != nil {
			return fmt.Errorf("can't render template %q: %w", gvIndexTemplate.Name(), err)
		}

		gvIndexFile := groupDir + groupFileNameSuffix
		err = os.WriteFile(gvIndexFile, data, 0777)
		if err != nil {
			return fmt.Errorf("can't write file %q: %w", gvIndexFile, err)
		}
		klog.V(2).InfoS("Created group index file", "Path", gvIndexFile)

		for _, resourceInfo := range resourceInfos {
			data, err = assets.RenderTemplate(kindTemplate, resourceInfo)
			if err != nil {
				return fmt.Errorf("can't render template %q: %w", kindTemplate.Name(), err)
			}

			kindFile := filepath.Join(groupDir, resourceInfo.Names.Plural+kindFileNameSuffix)
			err = os.WriteFile(kindFile, data, 0777)
			if err != nil {
				return fmt.Errorf("can't write file %q: %w", kindFile, err)
			}
			klog.V(2).InfoS("Created kind file", "Path", kindFile)
		}
	}

	return nil
}
