# Install Operator

Install ScyllaDB Operator and its dependencies into your Kubernetes cluster. Choose the installation method that matches your environment:

- **[GitOps](install-with-gitops.md)** — install using `kubectl apply` with manifests from the project repository. Recommended for most environments.
- **[Helm](install-with-helm.md)** — install using Helm charts.
- **[OpenShift](install-on-openshift.md)** — install via the Operator Lifecycle Manager (OLM) software catalog on Red Hat OpenShift.

Before installing, review the [prerequisites](prerequisites.md) to ensure your environment meets all requirements.

:::{toctree}
:maxdepth: 1

prerequisites
install-with-gitops
install-with-helm
install-on-openshift
set-up-multi-dc-infrastructure
:::
