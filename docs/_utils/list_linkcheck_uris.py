#!/usr/bin/env python3
"""Lists the URIs the linkcheck builder would check in the docs of a git revision.

The revision is checked out into a temporary worktree and built with the linkcheck builder with every URI ignored, so
nothing is requested over the network, but the builder still records each URI it found in output.json. URIs are
collected through Sphinx rather than from the sources because some of them are only resolved at build time, e.g. from
MyST substitutions.

The script is run with the docs' Python environment, which builds the revision's sources with the current
dependencies.
"""

import argparse
import json
import os
import subprocess
import sys
import tempfile


def git(*args: str, cwd: str) -> str:
    return subprocess.run(['git', *args], cwd=cwd, check=True, capture_output=True, text=True).stdout.strip()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('revision', help='Git revision to list the URIs of.')
    parser.add_argument('source_dir', help='Docs source directory in the current checkout.')
    parser.add_argument('output', help='File to write the URIs to, one per line.')
    args = parser.parse_args()

    source_dir = os.path.abspath(args.source_dir)
    repo_root = git('rev-parse', '--show-toplevel', cwd=source_dir)
    source_dir_in_repo = os.path.relpath(source_dir, repo_root)
    commit = git('rev-parse', '--verify', f'{args.revision}^{{commit}}', cwd=repo_root)

    with tempfile.TemporaryDirectory() as tmp_dir:
        worktree_dir = os.path.join(tmp_dir, 'worktree')
        build_dir = os.path.join(tmp_dir, 'build')
        git('worktree', 'add', '--detach', worktree_dir, commit, cwd=repo_root)
        try:
            subprocess.run(
                [
                    sys.executable, '-m', 'sphinx',
                    '--quiet',
                    '--builder', 'linkcheck',
                    '--define', 'linkcheck_ignore=.*',
                    os.path.join(worktree_dir, source_dir_in_repo),
                    build_dir,
                ],
                check=True,
            )
        finally:
            git('worktree', 'remove', '--force', worktree_dir, cwd=repo_root)

        uris = set()
        with open(os.path.join(build_dir, 'output.json')) as f:
            for line in f:
                if line.strip():
                    uris.add(json.loads(line)['uri'])

    with open(args.output, 'w') as f:
        f.writelines(f'{uri}\n' for uri in sorted(uris))


if __name__ == '__main__':
    main()
