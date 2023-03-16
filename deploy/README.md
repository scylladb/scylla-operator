## Prerequisites

Scylla Operator has a Cert Manager dependency, you have to install it first. If you intend to use the managed monitoring you also need a Prometheus Operator. 
They can be installed via the following commands executed in the root directory of Scylla Operator repository:
```shell
kubectl apply --server-side -f ./examples/common/cert-manager.yaml
kubectl apply -n prometheus-operator --server-side -f ./examples/third-party/prometheus-operator/
```

Now you need to wait for them to register and start sucessfully:
```shell
kubectl wait --for condition=established crd/certificates.cert-manager.io crd/issuers.cert-manager.io
kubectl wait --for condition=established "$( find ./examples/third-party/prometheus-operator/ -name '*.crd.yaml' -printf '-f=%p\n' )"
kubectl -n cert-manager rollout status deployment.apps/cert-manager{,-cainjector,-webhook}
kubectl -n prometheus-operator rollout status deployment.apps/prometheus-operator
```

---

## Scylla Operator deployment

To deploy Scylla Operator, execute the following from root directory in the repository:
```shell
kubectl apply -n scylla-operator --server-side -f ./deploy/operator/
```

Before Scylla or Scylla Manager can be deployed, CRD must enter established mode and Scylla Operator needs to become ready.
To wait for it use following commands:
```shell
kubectl wait --for condition=established crd/scyllaclusters.scylla.scylladb.com
kubectl wait --for condition=established crd/nodeconfigs.scylla.scylladb.com
kubectl wait --for condition=established crd/scyllaoperatorconfigs.scylla.scylladb.com
kubectl -n scylla-operator rollout status deployment.apps/scylla-operator
kubectl -n scylla-operator rollout status deployment.apps/webhook-server
```

If you would like to customize Scylla Operator deployment, you can either edit manifests manually, 
or check out our Scylla Operator Helm chart.

---

## Scylla Manager deployment

There are two types of deployments, one dedicated for production deployments, second one for development purposes. \
The main difference between them are amount of resources allocated for Scylla Manager internal Scylla single node cluster. 

To deploy Scylla Manager, run following command from root directory in the repository:
#### Production environment
```shell
kubectl apply -f deploy/manager/prod
```

#### Development environment
```shell
kubectl apply -f deploy/manager/dev
```

Scylla Manager is ready to be used once internal Scylla cluster is up, and Manager is running. To wait for them execute:
```shell
kubectl -n scylla-manager rollout status statefulset.apps/scylla-manager-cluster-manager-dc-manager-rack
kubectl -n scylla-manager rollout status deployment.apps/scylla-manager-controller
```

If you would like to customize Scylla Manager, you can either edit manifests manually, 
or check out our Scylla Manager Helm chart.
