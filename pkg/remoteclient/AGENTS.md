# pkg/remoteclient

Generic multi-cluster Kubernetes client manager. Used by the operator to manage client connections to remote clusters (defined by `RemoteKubernetesCluster` CRs).

## Packages

| Package | Purpose |
|---|---|
| `pkg/remoteclient/client` | `ClusterClient[CT]` — per-cluster client registry |
| `pkg/remoteclient/informers` | `SharedInformerFactory` — per-cluster shared informers |
| `pkg/remoteclient/lister` | `GenericClusterLister` — per-cluster typed listers |

## Core concepts

- **Cluster key** — the `RemoteKubernetesCluster.Name` string. All APIs are keyed by this.
- **Config** — raw kubeconfig bytes from `Secret.Data[naming.KubeConfigSecretKey]`.
- **`DynamicClusterInterface`** — the common interface for receiving `UpdateCluster` / `DeleteCluster` events. Both `ClusterClient` and `SharedInformerFactory` implement it.

## Client API

```go
// Create a cluster client for a specific client type (e.g., kubernetes.Interface)
clusterClient := client.NewClusterClient(func(cfg []byte) (kubernetes.Interface, error) {
    restCfg, err := clientcmd.RESTConfigFromKubeConfig(cfg)
    // ...
    return kubernetes.NewForConfig(restCfg)
})

// Register or refresh a cluster (called by RemoteKubernetesCluster controller)
err := clusterClient.UpdateCluster("remote-cluster-name", kubeconfigBytes)

// Remove a cluster
clusterClient.DeleteCluster("remote-cluster-name")

// Get client for a specific cluster
kube, err := clusterClient.Cluster("remote-cluster-name")
```

`UpdateCluster` hashes the config bytes — if unchanged, the existing client is reused.

## Informer API

```go
// Create a shared informer factory
factory := remoteinformers.NewSharedInformerFactory[kubernetes.Interface](clusterClient, defaultResync)

// Create a GenericClusterInformer for a resource type
podInformer := factory.ForResource(&corev1.Pod{}, remoteinformers.ClusterListWatch[kubernetes.Interface]{
    ListFunc: func(ctx context.Context, cluster string, options metav1.ListOptions) (runtime.Object, error) {
        kube, err := clusterClient.Cluster(cluster)
        if err != nil { return nil, err }
        return kube.CoreV1().Pods(namespace).List(ctx, options)
    },
    WatchFunc: func(ctx context.Context, cluster string, options metav1.ListOptions) (watch.Interface, error) {
        kube, err := clusterClient.Cluster(cluster)
        if err != nil { return nil, err }
        return kube.CoreV1().Pods(namespace).Watch(ctx, options)
    },
})

// Add event handlers (applied to ALL clusters — including future ones)
podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{...})

// Start the factory
factory.Start(stopCh)

// Wait for caches to sync
factory.WaitForCacheSync(stopCh)
```

Handlers registered via `AddEventHandler` on an `UnboundInformer` automatically apply to new clusters as they are registered via `UpdateCluster`.

## Lister API

```go
// Build a cluster-aware lister from the informer's indexer
podLister := remotelister.NewClusterLister(
    corev1listers.NewPodLister,         // typed lister constructor
    podInformer.Indexer().Cluster,      // per-cluster indexer accessor
)

// Query remote cluster
pods, err := podLister.Cluster("remote-cluster-name").Pods(namespace).List(labels.Everything())
```

## Registration flow (how clusters get registered)

```
RemoteKubernetesCluster CR + Secret
  → RemoteKubernetesCluster controller (pkg/controller/remotekubernetescluster)
  → reads Secret.Data[naming.KubeConfigSecretKey]
  → calls dch.UpdateCluster(rkc.Name, kubeconfigBytes) for each DynamicClusterInterface
  → clients and informer factories receive the new cluster
```

On CR deletion: controller calls `dch.DeleteCluster(rkc.Name)`, factory stops per-cluster informers.

## Wiring a new remote client consumer

1. Create a `ClusterClient` for your client type and add it to the `dynamicClusterHandlers` slice passed to `NewRemoteKubernetesClusterController`.
2. Create a `SharedInformerFactory` wrapping that `ClusterClient` — also add it to `dynamicClusterHandlers`.
3. Call `factory.ForResource(...)` with a `ClusterListWatch` that uses `clusterClient.Cluster(name)`.
4. Register event handlers on the `GenericClusterInformer`.
5. Pass `factory` to `factory.Start(stopCh)` in the operator startup sequence.

The `RemoteKubernetesCluster` controller automatically calls `UpdateCluster` on all registered handlers when kubeconfigs appear or change.

## Files

| File | Key content |
|---|---|
| `pkg/remoteclient/client/client.go` | `ClusterClient[CT]`, `ClusterClientInterface`, `DynamicClusterInterface` |
| `pkg/remoteclient/informers/factory.go` | `sharedInformerFactory[CT]`, `UpdateCluster`/`DeleteCluster` semantics |
| `pkg/remoteclient/informers/factory_interfaces.go` | `SharedInformerFactory`, `UnboundInformer`, `GenericClusterInformer`, `ClusterListWatch` |
| `pkg/remoteclient/informers/generic.go` | `UnboundInformer` — binds to per-cluster `SharedIndexInformer` on demand |
| `pkg/remoteclient/lister/lister.go` | `ClusterIndexer`, `GenericClusterLister`, `NewClusterLister` |
| `pkg/controller/remotekubernetescluster/sync_dynamicclusterhandlers.go` | Canonical `UpdateCluster` call site |
| `pkg/controller/scylladbcluster/controller.go` | Canonical consumer of remote informers + listers |

## Important notes

- **`UnboundInformer`** allows registering handlers *before* the cluster exists. Handlers are applied retroactively when `UpdateCluster` is called.
- **Factory is a `DynamicClusterInterface`** — add it directly to `dynamicClusterHandlers` along with `ClusterClient` instances.
- **Cluster identity is `RemoteKubernetesCluster.Name`** — use it consistently as the cluster key everywhere.
- **Config is opaque bytes** — pass raw kubeconfig bytes as read from `Secret.Data[naming.KubeConfigSecretKey]`; the factory hashes them to detect changes.
