package managerv2

import (
	"fmt"
	"net/http"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/managerclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
)

type ClientsCache struct {
	clients map[string]*managerclient.Client
}

func MakeClientsCache() ClientsCache {
	return ClientsCache{clients: make(map[string]*managerclient.Client)}
}

func (c *ClientsCache) Get(sm *v1alpha1.ScyllaManager) (*managerclient.Client, error) {
	if client, ok := c.clients[naming.ManagerNamespaceAndName(sm)]; ok {
		return client, nil
	}

	return c.newClient(sm)
}

func (c *ClientsCache) newClient(sm *v1alpha1.ScyllaManager) (*managerclient.Client, error) {
	client, err := managerclient.NewClient(naming.ManagerAPIUrl(sm), &http.Transport{})
	if err != nil {
		return nil, fmt.Errorf("can't create new client to ScyllaManager: %s, in namespace: %s, %v", sm.Name, sm.Namespace, err)
	}

	clientRef := &client
	c.clients[naming.ManagerNamespaceAndName(sm)] = clientRef

	return clientRef, nil
}
