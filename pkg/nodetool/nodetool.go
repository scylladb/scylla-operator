package nodetool

import (
	"net/url"

	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
)

type Nodetool struct {
	client *scyllaclient.Client
}

func NewFromURL(u *url.URL) *Nodetool {
	return &Nodetool{url: u}
}
