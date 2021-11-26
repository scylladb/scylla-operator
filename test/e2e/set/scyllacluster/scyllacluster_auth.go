// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"
	"net/http"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaCluster authentication", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("agent requires authentication", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster")
		err := framework.SetupScyllaClusterSA(ctx, f.KubeClient().CoreV1(), f.KubeClient().RbacV1(), f.Namespace(), sc.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy")
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), sc, utils.GetMemberCount(sc))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc, di)

		framework.By("Rejecting an unauthorized request")

		_, hosts, err := utils.GetScyllaClient(ctx, f.KubeClient().CoreV1(), sc)

		emptyAuthToken := ""
		_, err = getScyllaClientStatus(ctx, hosts, emptyAuthToken)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(scyllaclient.StatusCodeOf(err)).To(o.Equal(http.StatusUnauthorized))

		framework.By("Accepting requests authorized using token from provisioned secret")

		tokenSecret, err := f.KubeClient().CoreV1().Secrets(sc.Namespace).Get(ctx, naming.AgentAuthTokenSecretName(sc.Name), metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		token, err := helpers.GetAgentAuthTokenFromSecret(tokenSecret)
		o.Expect(err).ToNot(o.HaveOccurred())

		_, err = getScyllaClientStatus(ctx, hosts, token)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Specifying auth token in agent config")

		agentConfig := struct {
			AuthToken string `yaml:"auth_token"`
		}{
			AuthToken: "42",
		}
		agentConfigData, err := yaml.Marshal(agentConfig)
		o.Expect(err).NotTo(o.HaveOccurred())

		agentConfigSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "agent-config",
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				naming.ScyllaAgentConfigFileName: agentConfigData,
			},
		}

		_, err = f.KubeClient().CoreV1().Secrets(f.Namespace()).Create(ctx, agentConfigSecret, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(`[{"op":"replace","path":"/spec/datacenter/racks/0/scyllaAgentConfig","value":"%s"}]`, agentConfigSecret.Name)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		// TODO: restart should be triggered by the Operator
		framework.By("Initiating a rolling restart")

		_, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.MergePatchType,
			[]byte(fmt.Sprintf(`{"spec": {"forceRedeploymentReason": "%s"}}`, "scyllaAgenConfig was updated to contain a token")),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to pick up token change")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc, di)

		framework.By("Accepting requests authorized using token from user agent config")
		_, err = getScyllaClientStatus(ctx, hosts, agentConfig.AuthToken)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Changing auth token in agent config")

		agentConfig.AuthToken = "666"
		agentConfigData, err = yaml.Marshal(agentConfig)
		o.Expect(err).NotTo(o.HaveOccurred())
		agentConfigSecret.Data[naming.ScyllaAgentConfigFileName] = agentConfigData
		_, err = f.KubeClient().CoreV1().Secrets(f.Namespace()).Update(ctx, agentConfigSecret, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Initiating a rolling restart")

		_, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.MergePatchType,
			[]byte(fmt.Sprintf(`{"spec": {"forceRedeploymentReason": "%s"}}`, "scyllaAgenConfig token was changed")),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to pick up token change")

		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc, di)

		framework.By("Accepting requests authorized using token from user agent config")
		_, err = getScyllaClientStatus(ctx, hosts, agentConfig.AuthToken)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

func getScyllaClientStatus(ctx context.Context, hosts []string, authToken string) (scyllaclient.NodeStatusInfoSlice, error) {
	cfg := scyllaclient.DefaultConfig(authToken, hosts...)

	client, err := scyllaclient.NewClient(cfg, log.NopLogger)
	o.Expect(err).NotTo(o.HaveOccurred())

	return client.Status(ctx, hosts[0])
}
