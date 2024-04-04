// Copyright (C) 2017 ScyllaDB

package rclone

import (
	"os"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/scylladb/go-reflectx"
	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/scylla-manager/v3/pkg/rclone/backend/localdir"
	"go.uber.org/multierr"
)

var providers = strset.New()

// HasProvider returns true iff provider was registered.
func HasProvider(name string) bool {
	return providers.Has(name)
}

// RegisterLocalDirProvider must be called before server is started.
// It allows for adding dynamically adding localdir providers.
func RegisterLocalDirProvider(name, description, rootDir string) error {
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		return errors.Wrapf(err, "register local dir provider %s", rootDir)
	}
	localdir.Init(name, description, rootDir)

	return errors.Wrap(registerProvider(name, name, LocalOptions{}), "register provider")
}

// MustRegisterLocalDirProvider calls RegisterLocalDirProvider and panics on
// error.
func MustRegisterLocalDirProvider(name, description, rootDir string) {
	if err := RegisterLocalDirProvider(name, description, rootDir); err != nil {
		panic(err)
	}
}

// RegisterS3Provider must be called before server is started.
// It allows for adding dynamically adding s3 provider named s3.
func RegisterS3Provider(opts S3Options) error {
	const (
		name    = "s3"
		backend = "s3"
	)

	opts.AutoFill()
	if err := opts.Validate(); err != nil {
		return err
	}

	return errors.Wrap(registerProvider(name, backend, opts), "register provider")
}

// MustRegisterS3Provider calls RegisterS3Provider and panics on error.
func MustRegisterS3Provider(provider, endpoint, accessKeyID, secretAccessKey string) {
	opts := DefaultS3Options()
	opts.Provider = provider
	opts.Endpoint = endpoint
	opts.AccessKeyID = accessKeyID
	opts.SecretAccessKey = secretAccessKey

	if err := RegisterS3Provider(opts); err != nil {
		panic(err)
	}
}

// RegisterGCSProvider must be called before server is started.
// It allows for adding dynamically adding gcs provider named gcs.
func RegisterGCSProvider(opts GCSOptions) error {
	const (
		name    = "gcs"
		backend = "gcs"
	)

	opts.AutoFill()

	return errors.Wrap(registerProvider(name, backend, opts), "register provider")
}

// RegisterAzureProvider must be called before server is started.
// It allows for adding dynamically adding gcs provider named gcs.
func RegisterAzureProvider(opts AzureOptions) error {
	const (
		name    = "azure"
		backend = "azureblob"
	)

	opts.AutoFill()

	return errors.Wrap(registerProvider(name, backend, opts), "register provider")
}

func registerProvider(name, backend string, options interface{}) error {
	var (
		m     = reflectx.NewMapper("yaml").FieldMap(reflect.ValueOf(options))
		extra = []string{"name=" + name}
		errs  error
	)

	// Set type
	errs = multierr.Append(errs, fs.ConfigFileSet(name, "type", backend))

	// Set and log options
	for key, rval := range m {
		if s := rval.String(); s != "" {
			errs = multierr.Append(errs, fs.ConfigFileSet(name, key, s))
			if strings.Contains(key, "secret") || strings.Contains(key, "key") {
				extra = append(extra, key+"="+strings.Repeat("*", len(s)))
			} else {
				extra = append(extra, key+"="+s)
			}
		}
	}

	// Check for errors
	if errs != nil {
		return errors.Wrapf(errs, "register %s provider", name)
	}

	providers.Add(name)
	fs.Infof(nil, "registered %s provider [%s]", name, strings.Join(extra, ", "))

	return nil
}
