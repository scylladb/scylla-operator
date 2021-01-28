# Scylla Operator


[![LICENSE](https://img.shields.io/github/license/scylladb/scylla-operator.svg)](https://github.com/scylladb/scylla-operator/blob/master/LICENSE)
[![Language](https://img.shields.io/badge/Language-Go-blue.svg)](https://golang.org/)
[![Go Report Card](https://goreportcard.com/badge/github.com/scylladb/scylla-operator)](https://goreportcard.com/report/github.com/scylladb/scylla-operator)
[![GitHub release](https://img.shields.io/github/tag/scylladb/scylla-operator.svg?label=release)](https://github.com/scylladb/scylla-operator/releases)

[Scylla Operator](https://github.com/scylladb/scylla-operator) is a Kubernetes Operator for managing and automating tasks related to managing a Scylla clusters.

[Scylla](https://www.scylladb.com) is a close-to-the-hardware rewrite of Cassandra in C++. It features a shared nothing architecture that enables true linear scaling and major hardware optimizations that achieve ultra-low latencies and extreme throughput. It is a drop-in replacement for Cassandra and uses the same interfaces.

![](logo.png)

## Quickstart

To quickly deploy a Scylla cluster, choose one of the following options:

* [Generic](docs/source/generic.md): Follow this guide for the general way to use the operator.
* [GKE](docs/source/gke.md): An advanced guide for deploying Scylla with the best performance settings on [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine).
* [EKS](docs/source/eks.md): An advanced guide for deploying Scylla with the best performance settings on [Amazon Elastic Kubernetes Service](https://aws.amazon.com/eks/).

You can also use Helm charts to deploy both Scylla Operator and Scylla clusters:

```bash
helm repo add scylla-operator https://storage.googleapis.com/scylla-operator-charts
```

## Description

> Kubernetes Operator for Scylla

Currently it supports:
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

Future additions include:
* Restore


## Top-Performance Setup

Scylla performs the best when it has fast disks and direct access to the CPU. To deploy Scylla with maximum performance, follow the guide for your environment:
* [GKE](docs/source/gke.md)
* [EKS](docs/source/eks.md)


## Bugs

If you find a bug or need help running scylla, you can reach out in the following ways:
* `#scylla-operator` channel on [Slack](https://scylladb-users-slackin.herokuapp.com/).
* File an [issue](https://github.com/scylladb/scylla-operator/issues) describing the problem and how to reproduce.

## Building the project

You can easily build Scylla Operator in your environment:
* Run `make docker-build` and wait for the image to be built.

## Contributing

We would **love** you to contribute to Scylla Operator, help make it even better and learn together! Use these resources to help you get started:
* `#scylla-operator` channel on [Slack](https://scylladb-users-slackin.herokuapp.com/).
* [Contributing Guide](docs/source/contributing.md)

## Acknowledgements

This project is based on the Apache Cassandra Operator, a community effort started by [yanniszark](https://github.com/yanniszark) of [Arrikto](https://www.arrikto.com/), as part of the [Rook project](https://rook.io/).


