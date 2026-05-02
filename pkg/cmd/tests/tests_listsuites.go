package tests

import (
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	ginkgotest "github.com/scylladb/scylla-operator/pkg/test/ginkgo"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

// NewListSuitesCommand returns a command that prints the names of all
// registered test suites, one per line, on stdout.
func NewListSuitesCommand(streams genericclioptions.IOStreams, testSuites ginkgotest.TestSuites) *cobra.Command {
	cmd := &cobra.Command{
		Use: "list-suites",
		Long: templates.LongDesc(`
		Prints the names of registered test suites, one per line.
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range testSuites.Names() {
				_, err := fmt.Fprintln(streams.Out, name)
				if err != nil {
					return fmt.Errorf("can't write suite name: %w", err)
				}
			}
			return nil
		},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	return cmd
}
