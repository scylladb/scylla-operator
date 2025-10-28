// Copyright (C) 2024 ScyllaDB

package version

import (
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/build"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/spf13/cobra"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubectl/pkg/util/templates"
)

type Options struct {
}

func NewOptions(streams genericclioptions.IOStreams) *Options {
	return &Options{}
}

func NewCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewOptions(streams)

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Prints version.",
		Long: templates.LongDesc(`
		version prints the program version.
		`),
		Example: templates.Examples(fmt.Sprintf(`
		# Print the program version
		version
		`)),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd)
			if err != nil {
				return err
			}

			return nil
		},
		ValidArgs: []string{},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	return cmd
}

func (o *Options) Validate() error {
	var errs []error

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *Options) Complete() error {
	return nil
}

func (o *Options) Run(genericclioptions.IOStreams, *cobra.Command) error {
	fmt.Printf("GitCommit=%s\n", build.GitCommit())
	return nil
}
