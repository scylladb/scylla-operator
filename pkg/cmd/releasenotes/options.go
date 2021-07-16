// Copyright (C) 2021 ScyllaDB

package releasenotes

import (
	"context"
	"fmt"
	"strings"

	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	githubql "github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/util/errors"
)

type GenerateOptions struct {
	genericclioptions.IOStreams

	OrganizationName string
	RepositoryName   string
	RepositoryPath   string

	GithubToken string

	ReleaseName string
	StartRef    string
	EndRef      string

	ghClient *githubql.Client
}

func NewGitGenerateOptions(streams genericclioptions.IOStreams) *GenerateOptions {
	return &GenerateOptions{
		IOStreams:        streams,
		OrganizationName: "scylladb",
		RepositoryName:   "scylla-operator",
		RepositoryPath:   ".",
	}
}

func (o *GenerateOptions) Validate() error {
	var errs []error

	if o.ReleaseName == "" {
		errs = append(errs, fmt.Errorf("release name can't be empty"))
	}

	if strings.HasPrefix(o.ReleaseName, "v") {
		errs = append(errs, fmt.Errorf("release name can't has 'v' prefix"))
	}

	if o.StartRef == "" {
		errs = append(errs, fmt.Errorf("start ref can't be empty"))
	}

	if o.EndRef == "" {
		errs = append(errs, fmt.Errorf("end ref can't be empty"))
	}

	if len(o.GithubToken) == 0 {
		errs = append(errs, fmt.Errorf("github-token can't be empty"))
	}

	return errors.NewAggregate(errs)
}

func (o *GenerateOptions) Complete(ctx context.Context) error {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: o.GithubToken},
	)
	httpClient := oauth2.NewClient(ctx, ts)
	o.ghClient = githubql.NewClient(httpClient)

	return nil
}
