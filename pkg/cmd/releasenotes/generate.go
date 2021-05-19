// Copyright (C) 2021 ScyllaDB

package releasenotes

import (
	"context"

	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

const (
	EnvVarPrefix = "SCYLLA_OPERATOR_GEN_RELEASE_NOTES_"
)

func NewGenGitReleaseNotesCommand(ctx context.Context, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewGitGenerateOptions(streams)

	cmd := &cobra.Command{
		Short: "Generates release notes based on merge commits from local git repository between two tags.",
		Long: templates.LongDesc(`
		Scylla Operator Release Notes

		This command generates release notes based on merge commits from local git repository between two tags.
		`),

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.ReadFlagsFromEnv(EnvVarPrefix, cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}

			if err := o.Complete(ctx); err != nil {
				return err
			}

			if err := o.Run(ctx); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&o.ReleaseName, "release-name", o.ReleaseName, "Name of the release.")
	cmd.Flags().StringVar(&o.RepositoryPath, "repository-path", o.RepositoryPath, "Path to the git repository.")
	cmd.Flags().StringVar(&o.StartRef, "start-ref", o.StartRef, "First commit reference, pull requests merged after this ref (including this ref) will be part of the release notes.")
	cmd.Flags().StringVar(&o.EndRef, "end-ref", o.EndRef, "Last commit reference, pull requests merged before (including this ref) this ref will be part of the release notes.")
	cmd.Flags().StringVar(&o.GithubToken, "github-token", o.GithubToken, "GitHub token used to authenticate requests.")

	return cmd
}
