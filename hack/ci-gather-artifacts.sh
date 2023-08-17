#!/bin/bash
#
# Copyright (C) 2021 ScyllaDB
#
# This script gathers artifacts to debug CI run later.
# WARNING: The dumps include sensitive data (like secrets)

set -euxEo pipefail

ARTIFACTS_DIR=${ARTIFACTS_DIR:-$( mktemp -d )}
mkdir -p "${ARTIFACTS_DIR}"
cd "${ARTIFACTS_DIR}"

kubectl version > kubectl.version

namespaced_resources=$( kubectl api-resources --namespaced=true --verbs=get,list -o name )
cluster_scoped_resources=$( kubectl api-resources --namespaced=false --verbs=list -o name )

mkdir cluster-scoped-resources
pushd cluster-scoped-resources
for r in ${cluster_scoped_resources}; do
  for f in yaml wide; do
    kubectl get --ignore-not-found=true "${r}" -o "${f}" > "${r}.${f}" &
  done
done
wait
popd

# Dump namespaced resources globally for easy querying.
mkdir namespaced-resources
pushd namespaced-resources
for r in ${namespaced_resources}; do
  for f in yaml wide; do
    # Ignore not found to tolerate objects with delayed deletion or TTL.
    kubectl get --ignore-not-found=true "${r}" -A -o "${f}" > "${r}.${f}" &
  done
done
wait
popd

mkdir namespaces
pushd namespaces
namespaces=$( kubectl get namespaces --template='{{ range .items }}{{ println .metadata.name }}{{ end }}' )
for n in ${namespaces}; do
    mkdir "${n}"
    pushd "${n}"

    for r in ${namespaced_resources}; do
        objects=$( kubectl -n "${n}" get "${r}" --template='{{ range .items }}{{ println .metadata.name }}{{ end }}' )

        if [[ -z "${objects}" ]]; then
            continue
        fi

        mkdir "${r}"
        pushd "${r}"
        for o in ${objects}; do
            # Ignore not found to tolerate objects with delayed deletion or TTL.
            kubectl -n "${n}" get --ignore-not-found=true "${r}"/"${o}" -o yaml > "${o}".yaml

            case "${r}" in
            "pods")
                mkdir "${o}"
                pushd "${o}"

                containers=$( kubectl -n "${n}" get "pods/${o}" --template='{{ range .spec.containers }}{{ println .name }}{{ end }}' )
                for c in ${containers}; do
                    # Logs are best effort, a pod might not have been running yet.
                    { kubectl -n "${n}" logs pod/"${o}" -c="${c}" > "${c}".current; } || true

                    has_previous=$( kubectl -n "${n}" get "pods/${o}" --template="{{ range .status.containerStatuses }}{{ if eq .name \"${c}\" }}{{ gt .restartCount 0 | print }}{{ end }}{{ end }}" )
                    if [[ "${has_previous}" = "true" ]]; then
                      { kubectl -n "${n}" logs pod/"${o}" -c="${c}" --previous > "${c}".previous; } || true
                    fi
                done
                popd
                ;;
            esac
        done
        popd
    done

    popd
done
popd

mkdir describe
pushd describe
# describe nodes provides valuable usage information based on the node and pods objects
kubectl describe nodes > nodes.desc
popd

echo "Artifacts stored into directory: ${ARTIFACTS_DIR}"
