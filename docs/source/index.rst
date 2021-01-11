=============================
Scylla Operator Documentation
=============================

.. toctree::
   :hidden:
   :maxdepth: 2

   generic
   eks
   gke
   manager
   node_operations
   upgrade
   scylla_cluster_crd
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
* Monitoring with Prometheus and Grafana

**Choose a topic to begin**:

* :doc:`Deploying Scylla on a Kubernetes Cluster <generic>`
* :doc:`Deploying Scylla on EKS (experimental) <eks>`
* :doc:`Deploying Scylla on GKE <gke>`
* :doc:`Deploying Scylla Manager on a Kubernetes Cluster <manager>`
* :doc:`Node operations <node_operations>`
* :doc:`Upgrade procedures <upgrade>`
* :doc:`Scylla Cluster Custom Resource Definition (CRD) <scylla_cluster_crd>`
* :doc:`Contributing to the Scylla Operator Project <contributing>`



