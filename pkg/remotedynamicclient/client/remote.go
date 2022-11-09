// Copyright (c) 2022 ScyllaDB.

package client

import (
	"crypto/sha512"
	"fmt"
	"sync"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

type RemoteInterface interface {
	Region(region string) (dynamic.Interface, error)
}

type Dynamic interface {
	Update(region string, config []byte) error
	Delete(region string)
}

type RemoteDynamicClient struct {
	clientFactory func(config []byte) (dynamic.Interface, error)

	lock             sync.RWMutex
	credentialHashes map[string]string
	regionClients    map[string]dynamic.Interface
}

type RemoteDynamicClientOption func(client *RemoteDynamicClient)

func WithCustomClientFactory(factoryFunc func(config []byte) (dynamic.Interface, error)) func(client *RemoteDynamicClient) {
	return func(client *RemoteDynamicClient) {
		client.clientFactory = factoryFunc
	}
}

func NewRemoteDynamicClient(options ...RemoteDynamicClientOption) *RemoteDynamicClient {
	rc := &RemoteDynamicClient{
		clientFactory: func(config []byte) (dynamic.Interface, error) {
			restConfig, err := clientcmd.RESTConfigFromKubeConfig(config)
			if err != nil {
				return nil, fmt.Errorf("can't create REST config from kubeconfig: %w", err)
			}

			rc, err := dynamic.NewForConfig(restConfig)
			if err != nil {
				return nil, fmt.Errorf("can't create client: %w", err)
			}

			return rc, nil
		},
		lock:             sync.RWMutex{},
		credentialHashes: make(map[string]string),
		regionClients:    make(map[string]dynamic.Interface),
	}

	for _, option := range options {
		option(rc)
	}

	return rc
}

func (c *RemoteDynamicClient) Update(region string, config []byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	credentialsHash := hash(config)
	if h, ok := c.credentialHashes[region]; ok && h == credentialsHash {
		return nil
	}

	klog.V(4).InfoS("Updating remote client", "region", region)

	regionClient, err := c.clientFactory(config)
	if err != nil {
		return fmt.Errorf("can't create client to region %q: %w", region, err)
	}

	c.regionClients[region] = regionClient
	c.credentialHashes[region] = credentialsHash

	return nil
}

func (c *RemoteDynamicClient) Delete(region string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	klog.V(4).InfoS("Removing remote client", "region", region)
	delete(c.regionClients, region)
	delete(c.credentialHashes, region)
}

func (c *RemoteDynamicClient) Region(region string) (dynamic.Interface, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	client, ok := c.regionClients[region]
	if !ok {
		return nil, fmt.Errorf("client for %q region not found", region)
	}
	return client, nil
}

func hash(obj []byte) string {
	hasher := sha512.New()

	// Hasher never returns error on Write
	_, _ = hasher.Write(obj)
	return string(hasher.Sum(nil))
}
