:::{warning}
Helm doesn't support managing CustomResourceDefinition resources ([#5871](https://github.com/helm/helm/issues/5871), [#7735](https://github.com/helm/helm/issues/7735)).
Helm only creates CRDs on the first install and never updates them, while keeping the CRDs up to date (with any update) is absolutely essential.
In order to update them, users have to do it manually every time.
:::
