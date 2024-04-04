// Copyright (C) 2017 ScyllaDB

package backupspec

import (
	"path"
	"regexp"
	"unsafe"

	"github.com/gocql/gocql"
	"github.com/pkg/errors"
)

// Provider specifies type of remote storage like S3 etc.
type Provider string

// Provider enumeration.
const (
	S3    = Provider("s3")
	GCS   = Provider("gcs")
	Azure = Provider("azure")
)

var providers = []Provider{S3, GCS, Azure}

// Providers returns a list of all supported providers as a list of strings.
func Providers() []string {
	return *(*[]string)(unsafe.Pointer(&providers))
}

var testProviders = []string{"testdata"}

// AddTestProvider adds a provider for unit testing purposes.
// The provider is not returned in Providers() call but you can parse a Location
// with a test provider.
func AddTestProvider(name string) {
	testProviders = append(testProviders, name)
}

func hasProvider(s string) bool {
	for i := range providers {
		if providers[i].String() == s {
			return true
		}
	}
	for i := range testProviders {
		if testProviders[i] == s {
			return true
		}
	}
	return false
}

func (p Provider) String() string {
	return string(p)
}

// MarshalText implements encoding.TextMarshaler.
func (p Provider) MarshalText() (text []byte, err error) {
	return []byte(p.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (p *Provider) UnmarshalText(text []byte) error {
	if s := string(text); !hasProvider(s) {
		return errors.Errorf("unrecognised provider %q", text)
	}
	*p = Provider(text)
	return nil
}

// Location specifies storage provider and container/resource for a DC.
type Location struct {
	DC       string   `json:"dc"`
	Provider Provider `json:"provider"`
	Path     string   `json:"path"`
}

// ErrInvalid means that location is not adhering to scylla manager required
// [dc:]<provider>:<bucket> format.
var ErrInvalid = errors.Errorf("invalid location, the format is [dc:]<provider>:<bucket> ex. s3:my-bucket, the bucket name must be DNS compliant")

// Providers require that resource names are DNS compliant.
// The following is a super simplified DNS (plus provider prefix)
// matching regexp.
var pattern = regexp.MustCompile(`^(([a-zA-Z0-9\-\_\.]+):)?([a-z0-9]+):([a-z0-9\-\.]+)$`)

// NewLocation first checks if location string conforms to valid pattern.
// It then returns the location split into three components dc, remote, and
// bucket.
func NewLocation(location string) (l Location, err error) {
	m := pattern.FindStringSubmatch(location)
	if m == nil {
		return Location{}, ErrInvalid
	}

	return Location{
		DC:       m[2],
		Provider: Provider(m[3]),
		Path:     m[4],
	}, nil
}

func (l Location) String() string {
	p := l.Provider.String() + ":" + l.Path
	if l.DC != "" {
		p = l.DC + ":" + p
	}
	return p
}

func (l Location) MarshalText() (text []byte, err error) {
	return []byte(l.String()), nil
}

func (l *Location) UnmarshalText(text []byte) error {
	m := pattern.FindSubmatch(text)
	if m == nil {
		return errors.Errorf("invalid location %q, the format is [dc:]<provider>:<path> ex. s3:my-bucket, the path must be DNS compliant", string(text))
	}

	if err := l.Provider.UnmarshalText(m[3]); err != nil {
		return errors.Wrapf(err, "invalid location %q", string(text))
	}

	l.DC = string(m[2])
	l.Path = string(m[4])

	return nil
}

func (l Location) MarshalCQL(_ gocql.TypeInfo) ([]byte, error) {
	return l.MarshalText()
}

func (l *Location) UnmarshalCQL(_ gocql.TypeInfo, data []byte) error {
	return l.UnmarshalText(data)
}

// RemoteName returns the rclone remote name for that location.
func (l Location) RemoteName() string {
	return l.Provider.String()
}

// RemotePath returns string that can be used with rclone to specify a path in
// the given location.
func (l Location) RemotePath(p string) string {
	r := l.RemoteName()
	if r != "" {
		r += ":"
	}
	return path.Join(r+l.Path, p)
}
