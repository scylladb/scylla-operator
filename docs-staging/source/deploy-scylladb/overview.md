# Overview

This section covers deploying and configuring ScyllaDB clusters on Kubernetes.

## Deployment paths

### Supported platform (production)

If you are running on a [supported platform](../reference/releases.md#supported-kubernetes-environments), follow a [reference deployment](reference-deployments/index.md).
Reference deployments are end-to-end guides that walk you through node preparation, operator configuration, and deploying a production-ready ScyllaDB cluster.

The reference deployment guides link to [Before you deploy](before-you-deploy/overview.md) for node preparation steps — you do not need to follow those pages separately.

### Generic or development cluster

If you want a quick development cluster on any Kubernetes distribution, use [Deploy your first cluster](deploy-your-first-cluster.md).
This guide deploys a minimal ScyllaDB cluster and is not intended for production use.
For production, complete the [Before you deploy](before-you-deploy/overview.md) steps first.

## Further configuration

After your cluster is running, see these guides for additional setup:

- [Set up networking](set-up-networking/index.md) — expose ScyllaDB outside the Kubernetes cluster.
- [Install ScyllaDB Manager](install-scylladb-manager.md) — enable automated backups, repairs, and restore.
- [Set up monitoring](set-up-monitoring/index.md) — integrate with Prometheus and Grafana.
- [Deploy a multi-datacenter cluster](deploy-multi-dc-cluster.md) — span ScyllaDB across multiple Kubernetes clusters.
- [Production checklist](production-checklist.md) — verify your deployment is production-ready.
