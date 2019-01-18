package nodetool

import (
	"github.com/yanniszark/go-nodetool/client"
	"net/http"
	"net/url"
)

const (
	storageServiceMBean = "org.apache.cassandra.db:type=StorageService"
)

type Nodetool struct {
	Client client.Interface
}

func New(client client.Interface) *Nodetool {
	return &Nodetool{
		Client: client,
	}
}

func NewFromURL(u *url.URL) *Nodetool {
	return New(client.New(u, &http.Client{}))
}
