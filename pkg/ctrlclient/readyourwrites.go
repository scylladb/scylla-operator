// Copyright (c) 2026 ScyllaDB.

package ctrlclient

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReadYourWritesClient is a client.Client whose cached reads observe the writes made through it: a read that follows
// a write in the same process returns the written state or newer. controller-runtime builds such a client from
// client.CacheOptions.EnableReadYourWritesConsistency; it hands it out as a plain client.Client, which this type
// tells apart from a cached client without the guarantee. ObjectControl takes it because its Get is what the
// callers use to decide from when the object is missing from their cache.
type ReadYourWritesClient struct {
	c client.Client
}

// NewReadYourWritesClient wraps c, which the caller vouches for: it has to come from a manager or cluster built
// with client.CacheOptions.EnableReadYourWritesConsistency. controllermanager.New is the one production caller and
// sets that option in the same function.
func NewReadYourWritesClient(c client.Client) ReadYourWritesClient {
	return ReadYourWritesClient{
		c: c,
	}
}

// Client returns the wrapped client.
func (c ReadYourWritesClient) Client() client.Client {
	return c.c
}
