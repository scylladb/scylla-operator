// Copyright (C) 2021 ScyllaDB

package releasenotes

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/spf13/cobra"
	"go.uber.org/automaxprocs/maxprocs"
	"k8s.io/klog/v2"
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
			_, err := maxprocs.Set(maxprocs.Logger(func(format string, v ...interface{}) {
				klog.V(2).Infof(format, v)
			}))
			if err != nil {
				return fmt.Errorf("can't set maxproc: %w", err)
			}

			err = cmdutil.ReadFlagsFromEnv(EnvVarPrefix, cmd)
			if err != nil {
				return fmt.Errorf("can't read flags from env: %w", err)
			}

			return nil
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
	cmd.Flags().StringVar(&o.PreviousReleaseName, "previous-release-name", o.PreviousReleaseName, "Name of the previous release.")
	cmd.Flags().StringVar(&o.RepositoryPath, "repository-path", o.RepositoryPath, "Path to the git repository.")
	cmd.Flags().StringVar(&o.ContainerImageName, "container-image-name", o.Repository, `Full name of the container image.`)
	cmd.Flags().StringVar(&o.Repository, "repository", o.Repository, `Name of repository in "owner/name" format.`)
	cmd.Flags().StringVar(&o.StartRef, "start-ref", o.StartRef, "First commit reference, pull requests merged after this ref (including this ref) will be part of the release notes.")
	cmd.Flags().StringVar(&o.EndRef, "end-ref", o.EndRef, "Last commit reference, pull requests merged before (including this ref) this ref will be part of the release notes.")
	cmd.Flags().StringVar(&o.GithubToken, "github-token", o.GithubToken, "GitHub token used to authenticate requests.")

	cmdutil.InstallKlog(cmd)

	return cmd
}
