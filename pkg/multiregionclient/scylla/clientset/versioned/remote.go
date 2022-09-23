// Copyright (c) 2022 ScyllaDB.

package versioned

import (
	"crypto/sha512"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

type RemoteInterface interface {
	Datacenter(datacenter string) (Interface, error)
}

type Dynamic interface {
	OnUpdate(datacenter string, config []byte) error
	OnDelete(datacenter string) error
}

type RemoteClientset struct {
	lock             sync.RWMutex
	localClient      *Clientset
	credentialHashes map[string]string
	clientsets       map[string]*Clientset
}

func hash(obj []byte) string {
	return string(sha512.New().Sum(obj))
}

func NewRemoteClientset(localClient *Clientset) *RemoteClientset {
	rc := &RemoteClientset{
		lock:             sync.RWMutex{},
		localClient:      localClient,
		credentialHashes: make(map[string]string),
		clientsets:       make(map[string]*Clientset),
	}

	return rc
}

var emptyHash = hash(nil)

func (c *RemoteClientset) OnUpdate(datacenter string, config []byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	credentialsHash := hash(config)
	if h, ok := c.credentialHashes[datacenter]; ok && h == credentialsHash {
		return nil
	}

	if equality.Semantic.DeepEqual(credentialsHash, emptyHash) {
		c.clientsets[datacenter] = c.localClient
		c.credentialHashes[datacenter] = credentialsHash
		return nil
	}

	klog.V(4).InfoS("Updating remote client", "datacenter", datacenter)

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(config)
	if err != nil {
		return fmt.Errorf("can't create REST config from kubeconfig: %w", err)
	}

	cs, err := NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("can't create client: %w", err)
	}

	c.clientsets[datacenter] = cs
	c.credentialHashes[datacenter] = credentialsHash

	return nil
}

func (c *RemoteClientset) OnDelete(datacenter string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	klog.V(4).InfoS("Removing remote client", "datacenter", datacenter)
	delete(c.clientsets, datacenter)
	delete(c.credentialHashes, datacenter)

	return nil
}

func (c *RemoteClientset) Datacenter(datacenter string) (Interface, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	client, ok := c.clientsets[datacenter]
	if !ok {
		return nil, fmt.Errorf("multiregion client for %q datacenter not found", datacenter)
	}
	return client, nil
}
