// Copyright (C) 2021 ScyllaDB

package helpers

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/naming"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

type AgentAuthTokenSecret struct {
	AuthToken string `yaml:"auth_token"`
}

func GetAgentAuthToken(ctx context.Context, client corev1client.CoreV1Interface, clusterName, namespace string) (string, error) {
	secret, err := client.Secrets(namespace).Get(ctx, naming.AgentAuthTokenSecretName(clusterName), metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get auth token secret: %w", err)
	}

	return GetAgentAuthTokenFromSecret(secret)
}

func GetAgentAuthTokenFromSecret(secret *corev1.Secret) (string, error) {
	data := &AgentAuthTokenSecret{}
	if err := yaml.Unmarshal(secret.Data[naming.ScyllaAgentAuthTokenFileName], data); err != nil {
		return "", fmt.Errorf("unmarshal agent auth token secret: %w", err)
	}

	return data.AuthToken, nil
}
