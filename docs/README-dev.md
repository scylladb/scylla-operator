# Building the docs

This project uses [Sphinx Theme](https://sphinx-theme.scylladb.com/) , but we have modified the base project to adapt it to our workflows.

## Local preview

To build the documentation locally, run `make setup` first to install its dependencies.

Then, you can build and preview the documentation by following the steps outlined in the [Quickstart guide](https://sphinx-theme.scylladb.com/stable/getting-started/quickstart.html).

## Local preview with Podman

Here is an example how you can start quickly using containers, similarly to how our CI runs it.
(This assumes you are located at the repository root.)

```bash
podman run -it --pull=Always --rm -v="$( pwd )/docs:/go/$( go list -m )/docs:Z" --workdir="/go/$( go list -m )/docs" -p 5500:5500 quay.io/scylladb/scylla-operator-images:poetry-1.5 bash -euExo pipefail -O inherit_errexit -c 'poetry install && make multiversionpreview'
```

Docs will be available at http://localhost:5500/ 

## Update dependencies

```bash
podman run -it --pull=Always --rm -v="$( pwd )/docs:/go/$( go list -m )/docs:Z" --workdir="/go/$( go list -m )/docs" quay.io/scylladb/scylla-operator-images:poetry-1.5 bash -euExo pipefail -O inherit_errexit -c 'poetry update'
```
