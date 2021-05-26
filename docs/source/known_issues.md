# Known issues

### Scylla Manager does not boot up on Minikube

If your Scylla Manager is failing to apply 8th migration (008_*), then apply fix for [TRUNCATE queries](#truncate-queries-does-not-work-on-minikube).

### TRUNCATE queries does not work on Minikube

The `TRUNCATE` queries requires [hairpinning](https://en.wikipedia.org/wiki/Hairpinning) to be enabled. On minikube this is disabled by default.

To fix it execute the following command:
```
minikube ssh sudo ip link set docker0 promisc on
```
