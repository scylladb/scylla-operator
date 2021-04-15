#!/bin/bash
#
# This file is open source software, licensed to you under the terms
# of the Apache License, Version 2.0 (the "License").  See the NOTICE file
# distributed with this work for additional information regarding copyright
# ownership.  You may not use this file except in compliance with the License.
#
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

set -euExo pipefail

. /etc/os-release

debian_packages=(
    ca-certificates
    git
    make
)

fedora_packages=(
    git-core
    golang-bin
    make

    # Unfortunately podman doesn't work very well with kubernetes. See
    # https://fedoramagazine.org/docker-and-fedora-32/ for how to get
    # docker to work on a current fedora.
    moby-engine
)

arch_packages=(
    git
    make
    go
)

if [ "$ID" = "fedora" ]
then
    sudo dnf install -y "${fedora_packages[@]}"
fi

if [ "$ID" = "arch" ]
then
    sudo pacman -S --noconfirm --needed "${arch_packages[@]}"
fi

if [ "$ID" = "ubuntu" ] || [ "$ID" = "debian" ]
then
    # If someone has a non-system installation of go, just use that.
    if ! which go
    then
        sudo apt-get install golang
    fi

    # Similarly for docker
    if ! which docker
    then
        sudo apt-get install -y docker.io
    fi

    sudo apt-get install -y "${debian_packages[@]}"
fi

go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1

GOPATH="$(go env GOPATH)"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

cd "$TMPDIR"

# Kubernetes commands don't work with "go get"
# https://github.com/kubernetes/kubernetes/issues/79384
git clone https://github.com/kubernetes/kubernetes.git
cd kubernetes
# v1.19.x requires go 1.15.0
git checkout v1.18.9
make WHAT=cmd/kube-apiserver
cp _output/bin/kube-apiserver "$GOPATH"/bin
cd ..

# Build etcd
git clone https://github.com/etcd-io/etcd.git
cd etcd
git checkout v3.4.15
go mod vendor
./build
cp ./bin/etcd "$GOPATH"/bin
