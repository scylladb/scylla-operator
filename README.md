# Scylla Operator

[![GitHub release](https://img.shields.io/github/tag/scylladb/scylla-operator.svg?label=release)](https://github.com/scylladb/scylla-operator/releases)
[![Go](https://github.com/scylladb/scylla-operator/actions/workflows/go.yaml/badge.svg?branch=master)](https://github.com/scylladb/scylla-operator/actions/workflows/go.yaml?query=branch%3Amaster)
[![Go Report Card](https://goreportcard.com/badge/github.com/scylladb/scylla-operator)](https://goreportcard.com/report/github.com/scylladb/scylla-operator)
[![Language](https://img.shields.io/badge/Language-Go-blue.svg)](https://golang.org/)
[![LICENSE](https://img.shields.io/github/license/scylladb/scylla-operator.svg)](https://github.com/scylladb/scylla-operator/blob/master/LICENSE)


[Scylla Operator](https://github.com/scylladb/scylla-operator) is a Kubernetes Operator for managing and automating tasks related to managing Scylla clusters.

[Scylla](https://www.scylladb.com) is a close-to-the-hardware rewrite of Cassandra in C++. It features a shared nothing architecture that enables true linear scaling and major hardware optimizations that achieve ultra-low latencies and extreme throughput. It is a drop-in replacement for Cassandra and uses the same interfaces.

![](logo.png)

## Deploying the Operator
For version requirements see the [releases page](https://operator.docs.scylladb.com/stable/support/releases.html).
You have two equivalent options:

### GitOps (`kubectl`)
Kubernetes manifests are located in the `deploy/` folder. To deploy the operator manually using Kubernetes manifests or to integrate it into your GitOps flow please follow [the GitOps (`kubectl`) installation guide](https://operator.docs.scylladb.com/stable/installation/gitops.html). 

### Helm Charts
You can also use Helm charts to deploy both Scylla Operator and Scylla clusters by following [the Helm installation guide](https://operator.docs.scylladb.com/stable/installation/helm.html).

#### Stable
```bash
helm repo add scylla-operator https://storage.googleapis.com/scylla-operator-charts/stable
```

#### Latest
```bash
helm repo add scylla-operator https://storage.googleapis.com/scylla-operator-charts/latest
```

## Features
* Deploying multi-zone clusters
* Scaling up or adding new racks
* Scaling down
* Monitoring with Prometheus and Grafana
* Integration with [Scylla Manager](https://docs.scylladb.com/operating-scylla/manager/)
* Dead node replacement
* Version Upgrade
* Backup
* Repairs
* Autohealing

## Documentation

Scylla Operator has a [documentation site](https://operator.docs.scylladb.com/stable/).

Topics include:
* [Architecture overview](https://operator.docs.scylladb.com/stable/architecture/overview).
* An understanding-oriented [installation section](https://operator.docs.scylladb.com/stable/installation/overview.html).
* Scylla Operator CRD [explanation](https://operator.docs.scylladb.com/stable/resources/overview.html) and [API reference](https://operator.docs.scylladb.com/stable/api-reference/index.html).
* [Support/troubleshooting overview](https://operator.docs.scylladb.com/stable/support/overview.html) including a [support matrix](https://operator.docs.scylladb.com/stable/support/releases.html#support-matrix).
* Performance-tuned instructions ("quickstarts") for:
    * [Creating a dedicated GKE cluster for ScyllaDB in Google Cloud](https://operator.docs.scylladb.com/stable/quickstarts/gke.html),
    * [Creating a dedicated EKS cluster for ScyllaDB in Amazon AWS](https://operator.docs.scylladb.com/stable/quickstarts/eks.html).

There is also a [Scylla University Lesson](https://university.scylladb.com/courses/scylla-operations/lessons/kubernetes-operator/) focused on Scylla Operator: Follow this lesson on Scylla University to learn more about the Operator and how to use it. The lesson includes some hands-on examples which you can run yourself. 

## Roadmap
<!---
TODO: Link a dedicated roadmap.
-->
* Restore

## Support
If you find a bug please refer to the [support page](https://operator.docs.scylladb.com/stable/support/overview.html). You can file a [GitHub issue](https://github.com/scylladb/scylla-operator/issues) as well.

We are also available on the `#scylla-operator` channel on the [ScyllaDB users Slack](https://scylladb-users-slackin.herokuapp.com/) if you have questions.

## Contributing
We would **love** you to contribute to Scylla Operator, help make it even better and learn together! Reach out to us on `#scylla-operator` channel on [the ScyllaDB Users Slack](https://scylladb-users-slackin.herokuapp.com/) if you have questions.

