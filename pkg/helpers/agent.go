// Copyright (C) 2021 ScyllaDB

package helpers

import (
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/naming"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

type agentAuthTokenSecret struct {
	AuthToken string `yaml:"auth_token"`
}

func ParseTokenFromConfig(data []byte) (string, error) {
	config := &agentAuthTokenSecret{}
	err := yaml.Unmarshal(data, config)
	if err != nil {
		return "", fmt.Errorf("can't unmarshal agent auth token secret: %w", err)
	}

	return config.AuthToken, nil
}

func GetAgentAuthTokenFromAgentConfigSecret(secret *corev1.Secret) (string, error) {
	configData, ok := secret.Data[naming.ScyllaAgentConfigFileName]
	if !ok {
		return "", fmt.Errorf("secret %q is missing %q data", naming.ObjRef(secret), naming.ScyllaAgentConfigFileName)
	}

	return ParseTokenFromConfig(configData)
}

func GetAgentAuthTokenFromSecret(secret *corev1.Secret) (string, error) {
	configData, ok := secret.Data[naming.ScyllaAgentAuthTokenFileName]
	if !ok {
		return "", fmt.Errorf("secret %q is missing %q data", naming.ObjRef(secret), naming.ScyllaAgentAuthTokenFileName)
	}

	return ParseTokenFromConfig(configData)
}

func GetAgentAuthTokenConfig(token string) ([]byte, error) {
	data, err := yaml.Marshal(&agentAuthTokenSecret{AuthToken: token})
	if err != nil {
		return nil, fmt.Errorf("can't marshal auth token: %w", err)
	}

	return data, nil
}
