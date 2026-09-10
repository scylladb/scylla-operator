# Deploy ScyllaDB

This section covers deploying and configuring ScyllaDB clusters on Kubernetes.

## Deployment paths

### Supported platform (production)

If you are running on a [supported platform](https://operator.docs.scylladb.com/master/reference/releases.md#supported-kubernetes-environments), follow a [reference deployment](https://operator.docs.scylladb.com/master/deploy-scylladb/reference-deployments/index.md).
Reference deployments are end-to-end guides that walk you through node preparation, operator configuration, and deploying a production-ready ScyllaDB cluster.

The reference deployment guides link to [Before you deploy](https://operator.docs.scylladb.com/master/deploy-scylladb/before-you-deploy/index.md) for node preparation steps — you do not need to follow those pages separately.

### Generic or development cluster

If you want a quick development cluster on any Kubernetes distribution, use [Deploy your first cluster](https://operator.docs.scylladb.com/master/deploy-scylladb/deploy-your-first-cluster.md).
This guide deploys a minimal ScyllaDB cluster and is not intended for production use.
For production, complete the [Before you deploy](https://operator.docs.scylladb.com/master/deploy-scylladb/before-you-deploy/index.md) steps first.

### Multi-datacenter cluster

To span a ScyllaDB cluster across several datacenters, follow [Deploy a multi-datacenter ScyllaDB cluster](https://operator.docs.scylladb.com/master/deploy-scylladb/deploy-multi-datacenter-cluster.md).
This requires multiple interconnected Kubernetes clusters — see [Provision infrastructure](https://operator.docs.scylladb.com/master/install-operator/provision-infrastructure/index.md) for guides on preparing them.

## Further configuration

After your cluster is running, see these guides for additional setup:

- [Set up networking](https://operator.docs.scylladb.com/master/deploy-scylladb/set-up-networking/index.md) — expose ScyllaDB outside the Kubernetes cluster.
- [Install ScyllaDB Manager](https://operator.docs.scylladb.com/master/deploy-scylladb/install-scylladb-manager.md) — enable automated backups, repairs, and restore.
- [Set up monitoring](https://operator.docs.scylladb.com/master/deploy-scylladb/set-up-monitoring/index.md) — integrate with Prometheus and Grafana.
- [Production checklist](https://operator.docs.scylladb.com/master/deploy-scylladb/production-checklist.md) — verify your deployment is production-ready.
