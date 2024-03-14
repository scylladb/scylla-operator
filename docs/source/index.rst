=============================
Scylla Operator Documentation
=============================

.. toctree::
   :hidden:
   :maxdepth: 1

   generic
   eks
   gke
   helm
   manager
   monitoring
   clients/index
   migration
   nodeoperations/index
   exposing
   multidc/index
   performance
   upgrade
   releases
   support/index
   api-reference/index
   contributing

Scylla Operator is an open source project which helps users of Scylla Open Source and Scylla Enterprise run Scylla on Kubernetes (K8s)
The Scylla operator manages Scylla clusters deployed to Kubernetes and automates tasks related to operating a Scylla cluster, like installation, out and downscale, rolling upgrades.

.. image:: logo.png
   :width: 200pt

For the latest status of the project, and reports issue, see the Github Project. Also check out the K8s Operator lesson on Scylla University.

scylla-operator is a Kubernetes Operator for managing Scylla clusters.

Currently it supports:

* Deploying multi-zone clusters
* Scaling up or adding new racks
* Scaling down
* Monitoring with Prometheus and Grafana
* Integration with `Scylla Manager <https://docs.scylladb.com/operating-scylla/manager/>`_
* Dead node replacement
* Version Upgrade
* Backup
* Repairs
* Autohealing

**Choose a topic to begin**:

* :doc:`Deploying Scylla on a Kubernetes Cluster <generic>`
* :doc:`Deploying Scylla on EKS <eks>`
* :doc:`Deploying Scylla on GKE <gke>`
* :doc:`Deploying Scylla Manager on a Kubernetes Cluster <manager>`
* :doc:`Deploying Scylla stack using Helm Charts <helm>`
* :doc:`Setting up Monitoring using Prometheus and Grafana <monitoring>`
* :doc:`Using ScyllaDB APIs <clients/index>`
* :doc:`Node operations <nodeoperations/index>`
* :doc:`Exposing ScyllaCluster to other networks <exposing>`
* :doc:`Deploying multi-datacenter ScyllaDB clusters in Kubernetes <multidc/index>`
* :doc:`Performance tuning [Experimental] <performance>`
* :doc:`Upgrade procedures <upgrade>`
* :doc:`Releases <releases>`
* :doc:`Support <support/index>`
* :doc:`API Reference <api-reference/index>`
* :doc:`Contributing to the Scylla Operator Project <contributing>`
