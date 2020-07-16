#!/usr/bin/env python3

import os
import sys
import logging
import tarfile
import requests
import argparse
import tempfile
import subprocess
import shlex
import os

log = logging.getLogger(__name__)

KUBEBUILDER_URL = "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v2.3.1/kubebuilder_2.3.1_linux_amd64.tar.gz"
KUSTOMIZE_URL = "https://github.com/kubernetes-sigs/kustomize/releases/download/v3.1.0/kustomize_3.1.0_linux_amd64"
GO_URL = "https://dl.google.com/go/go1.13.12.linux-amd64.tar.gz"
GORELEASER_URL = "https://github.com/goreleaser/goreleaser/releases/download/v0.129.0/goreleaser_Linux_x86_64.tar.gz"
CONTROLLER_GEN_PKG = "sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0"

def parse_args():
    parser = argparse.ArgumentParser(description="Install binary dependencies")
    parser.add_argument("output_dir", metavar="OUTPUT_DIR", type=str,
                        help="Where to place the binaries.")
    return parser.parse_args()


def download_to(url, output_path):
    with open(output_path, "wb") as f:
        res = requests.get(url)
        f.write(res.content)


def download_from_tar(url, output_dir, paths_inside_tar=[], flatten=True):
    if not isinstance(paths_inside_tar, list):
        paths_inside_tar = list(paths_inside_tar)
    with tempfile.NamedTemporaryFile() as tarball:
        download_to(url, tarball.name)
        tar = tarfile.open(tarball.name)

        members = [tar.getmember(p) for p in paths_inside_tar]
        if not paths_inside_tar:
            members = tar.getmembers()

        for m in members:
            if flatten:
                m.name = os.path.basename(m.name)
            tar.extract(m, output_dir)

def download_go_package(pkg, output_dir):
    env = os.environ.copy()
    env["GOBIN"] = os.path.abspath(output_dir)
    p = subprocess.Popen(shlex.split("go get {}".format(pkg)), env=env)
    p.wait()

def main():
    global log
    logging.basicConfig(level=logging.INFO)

    args = parse_args()

    log.info("Installing kubebuilder...")
    download_from_tar(KUBEBUILDER_URL, args.output_dir)

    log.info("Installing kustomize...")
    kustomize_path = os.path.join(args.output_dir, "kustomize")
    download_to(KUSTOMIZE_URL, kustomize_path)
    os.chmod(kustomize_path, 755)

    log.info("Installing go...")
    download_from_tar(GO_URL, args.output_dir, flatten=False)

    log.info("Installing controller-gen...")
    download_go_package(CONTROLLER_GEN_PKG, args.output_dir)

    log.info("Installing goreleaser...")
    download_from_tar(GORELEASER_URL, args.output_dir,
                      paths_inside_tar=["goreleaser"])


if __name__ == "__main__":
    main(sys.exit(main()))
