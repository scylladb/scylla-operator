# Scylla Operator

[![GitHub release](https://img.shields.io/github/tag/scylladb/scylla-operator.svg?label=release)](https://github.com/scylladb/scylla-operator/releases)
[![Go](https://github.com/scylladb/scylla-operator/actions/workflows/go.yaml/badge.svg?branch=master)](https://github.com/scylladb/scylla-operator/actions/workflows/go.yaml?query=branch%3Amaster)
[![Go Report Card](https://goreportcard.com/badge/github.com/scylladb/scylla-operator)](https://goreportcard.com/report/github.com/scylladb/scylla-operator)
[![Language](https://img.shields.io/badge/Language-Go-blue.svg)](https://golang.org/)
[![LICENSE](https://img.shields.io/github/license/scylladb/scylla-operator.svg)](https://github.com/scylladb/scylla-operator/blob/master/LICENSE)


[Scylla Operator](https://github.com/scylladb/scylla-operator) is a Kubernetes Operator for managing and automating tasks related to managing a Scylla clusters.

[Scylla](https://www.scylladb.com) is a close-to-the-hardware rewrite of Cassandra in C++. It features a shared nothing architecture that enables true linear scaling and major hardware optimizations that achieve ultra-low latencies and extreme throughput. It is a drop-in replacement for Cassandra and uses the same interfaces.

![](logo.png)

## Deploying the Operator
The current version of scylla-operator requires Kubernetes >= 1.19.

### GitOps
Kubernetes manifests are located in the `deploy/` folder. To deploy the operator manually using Kubernetes manifests or to integrate it into your GitOps flow please follow [these instructions](./deploy/README.md). 

### Helm Charts
You can also use Helm charts to deploy both Scylla Operator and Scylla clusters:

#### Stable
```bash
helm repo add scylla-operator https://storage.googleapis.com/scylla-operator-charts/stable
```

#### Latest
```bash
helm repo add scylla-operator https://storage.googleapis.com/scylla-operator-charts/latest
```


## Quickstarts
To quickly deploy a ScyllaCluster, you can choose one of the following options:

* [Generic](docs/source/generic.md): Follow this guide for the general way to use the operator.
* [GKE](docs/source/gke.md): An advanced guide for deploying Scylla with the **best performance settings** on [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine).
* [EKS](docs/source/eks.md): An advanced guide for deploying Scylla with the **best performance settings** on [Amazon Elastic Kubernetes Service](https://aws.amazon.com/eks/).

## Releases
To find out more about our releases and how our CI/CD is setup there is a [dedicated docs page](./docs/source/releases.md).

## Documentation
Scylla Operator documentation is available on https://operator.docs.scylladb.com

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

## Roadmap
<!---
TODO: Link a dedicated roadmap.
-->
* Restore

## Support
If you find a bug please file an [issue](https://github.com/scylladb/scylla-operator/issues) for us.

We are also available on `#scylla-operator` channel on [Slack](https://scylladb-users-slackin.herokuapp.com/) if you have questions.

## Contributing
We would **love** you to contribute to Scylla Operator, help make it even better and learn together! Have a look at the [Contributing Guide](docs/source/contributing.md) or reach out to us on `#scylla-operator` channel on [Slack](https://scylladb-users-slackin.herokuapp.com/) if you have questions.
