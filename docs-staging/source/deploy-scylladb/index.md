# Deploy ScyllaDB

Deploy and configure ScyllaDB clusters on Kubernetes. Start with a quick single-cluster deployment, or follow a platform-specific reference deployment that covers the complete setup from scratch.

**Preparing your environment?** Start with [Before you deploy](before-you-deploy/index.md) — dedicated node pools, performance tuning, CPU pinning, and Operator configuration.

**Already have the Operator installed?** Go straight to [Deploy your first cluster](deploy-your-first-cluster.md).

**Starting from scratch on a specific platform?** Follow a reference deployment:

- [Reference deployment: GKE](reference-deployment-gke.md) — end-to-end on Google Kubernetes Engine.
- [Reference deployment: EKS](reference-deployment-eks.md) — end-to-end on Amazon EKS.
- [Reference deployment: OpenShift](reference-deployment-openshift.md) — end-to-end on Red Hat OpenShift.

:::{toctree}
:maxdepth: 1

before-you-deploy/index
deploy-your-first-cluster
reference-deployment-gke
reference-deployment-eks
reference-deployment-openshift
deploy-multi-dc-cluster
set-up-networking/index
set-up-monitoring
expose-grafana
production-checklist
:::
