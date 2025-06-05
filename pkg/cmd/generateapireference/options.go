package generateapireference

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type GenerateAPIRefsOptions struct {
	CustomResourceDefinitionPaths []string
	TemplatesDir                  string
	OutputDir                     string
	Overwrite                     bool
}

func NewGenerateAPIRefsOptions() *GenerateAPIRefsOptions {
	return &GenerateAPIRefsOptions{
		CustomResourceDefinitionPaths: nil,
		OutputDir:                     "",
	}
}

func (o *GenerateAPIRefsOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&o.TemplatesDir, "templates-dir", "", o.TemplatesDir, "A directory containing docs templates.")
	cmd.PersistentFlags().StringVarP(&o.OutputDir, "output-dir", "", o.OutputDir, "A directory where the generated files should be stored.")
	cmd.PersistentFlags().BoolVarP(&o.Overwrite, "overwrite", "", o.Overwrite, "Allows writing to output dir that already contains data. Existing files will be overwritten.")
}

func (o *GenerateAPIRefsOptions) Validate(args []string) error {
	var errs []error

	if len(args) == 0 {
		errs = append(errs, fmt.Errorf("at least one CRD has to be specified"))
	}

	if len(o.TemplatesDir) == 0 {
		errs = append(errs, fmt.Errorf("templates-dir path can't be empty"))
	}

	if len(o.OutputDir) > 0 {
		files, err := os.ReadDir(o.OutputDir)
		if err == nil {
			if len(files) > 0 && !o.Overwrite {
				errs = append(errs, fmt.Errorf("output directory %q is not empty and overwrite isn't enabled", o.OutputDir))
			}
		} else if !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("can't read output-dir %q: %w", o.OutputDir, err))
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *GenerateAPIRefsOptions) Complete(args []string) error {
	o.CustomResourceDefinitionPaths = args

	return nil
}
