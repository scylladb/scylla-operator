// Copyright (C) 2021 ScyllaDB

package cluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/controllers/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/resourceapply"
	"gopkg.in/yaml.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

type agentConfig struct {
	AuthToken string `yaml:"auth_token"`
}

func (cc *ClusterReconciler) syncAgentAuthToken(ctx context.Context, cluster *scyllav1.ScyllaCluster) error {
	secretName := naming.AgentAuthTokenSecretName(cluster.Name)
	var token string

	existing, err := cc.KubeClient.CoreV1().Secrets(cluster.Namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get auth token secret: %w", err)
	}

	if apierrors.IsNotFound(err) {
		token = rand.String(128)
	} else {
		token, err = helpers.GetAgentAuthTokenFromSecret(existing)
		if err != nil {
			return fmt.Errorf("get token from secret: %w", err)
		}
	}

	userToken, err := getAgentAuthTokenFromUserSecret(ctx, cc.KubeClient.CoreV1(), cluster)
	if err != nil {
		return fmt.Errorf("get auth token from user secret: %w", err)
	}

	// User token is preferred.
	if userToken != "" {
		token = userToken
	}

	secret, err := resource.MakeAgentAuthTokenSecret(cluster, token)
	if err != nil {
		return fmt.Errorf("make auth token secret: %w", err)
	}

	if err := resourceapply.ApplySecret(ctx, cc.Recorder, cc.Client, secret); err != nil {
		return fmt.Errorf("apply secret: %w", err)
	}

	return nil
}

func getAgentAuthTokenFromUserSecret(ctx context.Context, client corev1client.CoreV1Interface, cluster *scyllav1.ScyllaCluster) (string, error) {
	if len(cluster.Spec.Datacenter.Racks) == 0 {
		return "", nil
	}

	agentSecretName := cluster.Spec.Datacenter.Racks[0].ScyllaAgentConfig
	if agentSecretName == "" {
		return "", nil
	}

	secret, err := client.Secrets(cluster.Namespace).Get(ctx, agentSecretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("can't get secret %q: %w", agentSecretName, err)
	}

	cfg := &agentConfig{}
	if err := yaml.Unmarshal(secret.Data[naming.ScyllaAgentConfigFileName], &cfg); err != nil {
		return "", fmt.Errorf("can't parse agent config: %w", err)
	}

	return cfg.AuthToken, nil
}
