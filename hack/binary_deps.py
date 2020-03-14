#!/usr/bin/env python3

import os
import sys
import logging
import tarfile
import requests
import argparse
import tempfile

log = logging.getLogger(__name__)

KUBEBUILDER_URL = "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v1.0.5/kubebuilder_1.0.5_linux_amd64.tar.gz"
KUSTOMIZE_URL = "https://github.com/kubernetes-sigs/kustomize/releases/download/v2.0.3/kustomize_2.0.3_linux_amd64"
DEP_URL = "https://github.com/golang/dep/releases/download/v0.5.4/dep-linux-amd64"
GO_URL = "https://dl.google.com/go/go1.12.17.linux-amd64.tar.gz"


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


def main():
    global log
    logging.basicConfig(level=logging.INFO)

    args = parse_args()

    log.info("Installing kubebuilder...")
    download_from_tar(KUBEBUILDER_URL, args.output_dir)

    log.info("Installing kustomize...")
    download_to(KUSTOMIZE_URL, os.path.join(args.output_dir, "kustomize"))
    os.chmod(os.path.join(args.output_dir, "kustomize"), 755)

    log.info("Installing dep...")
    download_to(DEP_URL, os.path.join(args.output_dir, "dep"))
    os.chmod(os.path.join(args.output_dir, "dep"), 755)

    log.info("Installing go...")
    download_from_tar(GO_URL, args.output_dir, flatten=False)


if __name__ == "__main__":
    main(sys.exit(main()))
