# Troubleshooting installation issues

## Webhooks
Scylla Operator provides several custom API resources that use webhooks to function properly.

Unfortunately, it is often the case that user's clusters have modified SDN, that doesn't extend to the control plane, and Kubernetes apiserver is not able to reach the pods that serve the webhook traffic.
Another common case are firewall rules that block the webhook traffic.

:::{note}
   To be called a Kubernetes cluster, clusters are required to pass Kubernetes conformance test suite.
   This suite includes tests that require Kubernetes apiserver to be able to reach webhook services.
:::

:::{note}
   Before filing an issue, please make sure your cluster webhook traffic can reach your webhook services, independently of Scylla Operator resources.
:::

### EKS

#### Custom CNI
EKS is currently breaking Kubernetes webhooks [when used with custom CNI networking](https://github.com/aws/containers-roadmap/issues/1215).

:::{note}
   We advise you to avoid using such setups and use a conformant Kubernetes cluster that supports webhooks.
:::

There are some workarounds where you can reconfigure the webhook to use Ingress or hostNetwork instead, but it's beyond a standard configuration that we support and not specific to the Scylla Operator.

### GKE

#### Private clusters

If you use GKE private clusters you need to manually configure the firewall to allow webhook traffic.
You can find more information on how to do that in [GKE private clusters docs](https://cloud.google.com/kubernetes-engine/docs/how-to/private-clusters#add_firewall_rules).
