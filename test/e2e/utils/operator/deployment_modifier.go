// Copyright (C) 2026 ScyllaDB

package operator

import (
	"context"
	"fmt"
	"slices"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils/platform"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace      = "scylla-operator"
	deploymentName = "scylla-operator"
	containerName  = "scylla-operator"
)

// OperatorDeploymentModifier changes the installed operator Deployment, so the change persists regardless of how the
// operator was installed.
type OperatorDeploymentModifier interface {
	// SetFlags sets the given operator flags to the given values and waits for the operator to roll out.
	SetFlags(ctx context.Context, flags map[string]string)
	// UnsetFlags removes the given operator flags, so the operator falls back to their defaults, and waits for the
	// operator to roll out.
	UnsetFlags(ctx context.Context, names ...string)
}

// NewOperatorDeploymentModifier returns the OperatorDeploymentModifier for the platform the operator was installed on.
// It snapshots the object it is going to change and registers its restoration with the framework cleaners.
func NewOperatorDeploymentModifier(ctx context.Context, f *framework.Framework) OperatorDeploymentModifier {
	g.GinkgoHelper()

	switch p := platform.DetectPlatform(ctx, f.KubeAdminClient()); p {
	case platform.PlatformOLM:
		return newOLMOperatorDeploymentModifier(ctx, f)
	case platform.PlatformOther:
		return newManifestsOperatorDeploymentModifier(ctx, f)
	default:
		g.Fail(fmt.Sprintf("unsupported platform %q", p))
		return nil
	}
}

// flagEnvVarName returns the name of the environment variable the operator reads the flag from. The modifiers set the
// flags through environment variables, because a Subscription can't change the arguments of an operator installed by
// OLM.
func flagEnvVarName(name string) string {
	return cmdutil.NormalizeNameForEnvVar(naming.OperatorEnvVarPrefix + name)
}

func lookupFlagEnvVar(container *corev1.Container, name string) (string, bool) {
	idx := slices.IndexFunc(container.Env, func(envVar corev1.EnvVar) bool {
		return envVar.Name == flagEnvVarName(name)
	})
	if idx == -1 {
		return "", false
	}
	return container.Env[idx].Value, true
}

// expectFlagsNotPassedAsArgs fails if any of the flags is passed to the operator container as an argument, which the
// operator would read instead of the flag's environment variable.
func expectFlagsNotPassedAsArgs(container *corev1.Container, set map[string]string, unset []string) {
	g.GinkgoHelper()

	for _, name := range append(sortedKeys(set), unset...) {
		o.Expect(container.Args).NotTo(o.ContainElement(o.HavePrefix("--"+name+"=")), "operator container arguments")
	}
}

// setFlagEnvVars returns env with the environment variables of the set flags set to the given values and those of the
// unset flags removed.
func setFlagEnvVars(env []corev1.EnvVar, set map[string]string, unset []string) []corev1.EnvVar {
	names := append(sortedKeys(set), unset...)
	env = slices.DeleteFunc(slices.Clone(env), func(envVar corev1.EnvVar) bool {
		return slices.ContainsFunc(names, func(name string) bool {
			return envVar.Name == flagEnvVarName(name)
		})
	})
	for _, name := range sortedKeys(set) {
		env = append(env, corev1.EnvVar{
			Name:  flagEnvVarName(name),
			Value: set[name],
		})
	}
	return env
}

// waitForOperatorFlags waits until the operator container has the environment variables of the set flags with the
// given values and none of the unset ones, and the operator Deployment rolls it out.
func waitForOperatorFlags(ctx context.Context, f *framework.Framework, set map[string]string, unset []string) {
	g.GinkgoHelper()

	framework.By("Waiting for the operator Deployment to roll out with the updated flags")
	waitForOperatorContainer(ctx, f, func(container *corev1.Container) bool {
		for name, value := range set {
			v, ok := lookupFlagEnvVar(container, name)
			if !ok || v != value {
				return false
			}
		}

		for _, name := range unset {
			if _, ok := lookupFlagEnvVar(container, name); ok {
				return false
			}
		}

		return true
	})
}

// waitForOperatorContainer waits until the operator container satisfies the condition and the operator Deployment
// rolls it out.
func waitForOperatorContainer(ctx context.Context, f *framework.Framework, condition func(container *corev1.Container) bool) {
	g.GinkgoHelper()

	waitCtx, waitCtxCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer waitCtxCancel()
	_, err := controllerhelpers.WaitForDeploymentState(
		waitCtx,
		f.KubeAdminClient().AppsV1().Deployments(namespace),
		deploymentName,
		controllerhelpers.WaitForStateOptions{},
		func(deploy *appsv1.Deployment) (bool, error) {
			container, err := getOperatorContainer(deploy)
			if err != nil {
				return false, err
			}
			return condition(container), nil
		},
		controllerhelpers.IsDeploymentRolledOut,
		// Make sure no operator replica with the previous configuration is left to reconcile.
		func(deploy *appsv1.Deployment) (bool, error) {
			return deploy.Status.Replicas == *deploy.Spec.Replicas, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// addRestoringCleaner snapshots the object the modifier changes, which is either the operator Deployment or the OLM
// Subscription that installed the operator, and registers its restoration with the framework cleaners. After restoring
// it, the cleaner waits until the operator Deployment rolls out with the operator container's env from before the
// test, so that the following tests don't run against the modified operator.
func addRestoringCleaner(ctx context.Context, f *framework.Framework, resourceInfo collect.ResourceInfo, objectNamespace, objectName string) {
	g.GinkgoHelper()

	deploy, err := f.KubeAdminClient().AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	container, err := getOperatorContainer(deploy)
	o.Expect(err).NotTo(o.HaveOccurred())
	env := container.Env

	f.AddCleaners(framework.NewRestoringCleaner(
		ctx,
		f.AdminClientConfig(),
		f.KubeAdminClient(),
		f.DynamicAdminClient(),
		resourceInfo,
		objectNamespace,
		objectName,
		framework.RestoreStrategyUpdate,
		framework.WithPostCleanupHook(func(ctx context.Context) {
			framework.By("Waiting for the operator Deployment to roll out with the restored env")
			waitForOperatorContainer(ctx, f, func(container *corev1.Container) bool {
				return apiequality.Semantic.DeepEqual(container.Env, env)
			})
		}),
	))
}

func getOperatorContainerIndex(deploy *appsv1.Deployment) (int, error) {
	idx := slices.IndexFunc(deploy.Spec.Template.Spec.Containers, func(c corev1.Container) bool {
		return c.Name == containerName
	})
	if idx == -1 {
		return -1, fmt.Errorf("container %q not found in Deployment %q", containerName, deploymentName)
	}
	return idx, nil
}

func getOperatorContainer(deploy *appsv1.Deployment) (*corev1.Container, error) {
	idx, err := getOperatorContainerIndex(deploy)
	if err != nil {
		return nil, err
	}
	return &deploy.Spec.Template.Spec.Containers[idx], nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
