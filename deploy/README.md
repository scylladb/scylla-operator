# Scylla Operator deployment

Manifest files of Scylla Operator can be found in `operator` directory.

Note: Scylla Operator has a Cert Manager dependency, you have to install it first. Check documentation for details.

To deploy Scylla Operator, execute the following from root directory in the repository:
```shell
kubectl apply -fdeploy/operator
```


# Scylla Manager deployment

Manifest files of Scylla Manager can be found in `manager` directory.
You have to install Scylla Operator prior deploying Scylla Manager.

To deploy Scylla Manager, simply run following command from root directory in the repository:
```shell
kubectl apply -fdeploy/manager
```
