"""
Copyright (C) 2026 ScyllaDB

Sphinx extension to extend myst_substitutions with version context in build time.
"""

import os
import re
import subprocess
from typing import Any, Optional

import yaml
from sphinx.application import Sphinx
from sphinx.config import Config
from sphinx.errors import ExtensionError


def get_versions_from_config(repo_root: str) -> dict[str, str]:
    """
    Read version information from assets/config/config.yaml.

    Args:
        repo_root: Path to the repository root directory. Should point to the version-specific
                   directory (sphinx-multiversion temporary directory when building multi-version docs).

    Returns:
        Dictionary with versions extracted from config.yaml.
    """
    config_path = os.path.join(repo_root, 'assets', 'config', 'config.yaml')

    if not os.path.exists(config_path):
        raise ExtensionError(f"Config file not found: {config_path}")

    try:
        with open(config_path, 'r') as f:
            config_data = yaml.safe_load(f)

        operator = config_data.get('operator', {})

        scylla_db_version = operator.get('scyllaDBVersion')
        if not scylla_db_version:
            raise ExtensionError("operator.scyllaDBVersion not found in config.yaml")

        scylla_db_manager_agent_version = operator.get('scyllaDBManagerAgentVersion')
        if not scylla_db_manager_agent_version:
            raise ExtensionError("operator.scyllaDBManagerAgentVersion not found in config.yaml")

        # Strip digest if present (e.g., "3.8.0@sha256:..." -> "3.8.0")
        def strip_digest(version: str) -> str:
            return version.split('@')[0] if '@' in version else version

        return {
            'scyllaDBVersion': strip_digest(scylla_db_version),
            'scyllaDBManagerAgentVersion': strip_digest(scylla_db_manager_agent_version),
        }

    except yaml.YAMLError as e:
        raise ExtensionError(f"Failed to parse config.yaml: {e}")
    except Exception as e:
        raise ExtensionError(f"Error reading config.yaml: {e}")


def get_latest_release_for_revision(revision: str, repo_root: str) -> Optional[str]:
    """
    Get the latest non-prerelease version for a given branch/revision using git tags.

    Args:
        revision: Branch name (e.g., 'master', 'v1.19', 'v1.18')
        repo_root: Path to the git repository root directory. Should point to the original
                   checked-out repository (not the sphinx-multiversion temporary directory).

    Returns:
        Latest release tag (e.g., 'v1.19.0') or None if no matching release is found.
    """
    try:
        tags_output = subprocess.check_output(
            ['git', 'tag', '--list'],
            text=True,
            timeout=5,
            cwd=repo_root,
            stderr=subprocess.PIPE
        )
        all_tags = [tag.strip() for tag in tags_output.strip().split('\n') if tag.strip()]

        if not all_tags:
            raise ExtensionError(f"No git tags found in repository for revision {revision}")

        # Filter out pre-releases.
        stable_tags = [tag for tag in all_tags if re.match(r'^v\d+\.\d+\.\d+$', tag)]

        if not stable_tags:
            raise ExtensionError(f"No stable release tags found for revision {revision}. All tags are pre-releases.")

        # Sort tags by semantic version.
        stable_tags.sort(key=lambda x: [int(y) for y in x.lstrip('v').split('.')], reverse=True)

        # For versioned branches (vX.Y), try to find matching releases.
        if revision != "master":
            match = re.match(r'^v(\d+\.\d+)$', revision)
            if not match:
                raise ExtensionError(f"Could not parse version from revision '{revision}'. Expected format: 'master' or vX.Y")

            major_minor = match.group(1)
            matching_tags = [tag for tag in stable_tags if re.match(rf'^v{re.escape(major_minor)}\.', tag)]

            if matching_tags:
                return matching_tags[0]

        # For master or branches with no stable releases, return the latest stable release.
        return stable_tags[0]

    except subprocess.CalledProcessError as e:
        error_msg = e.stderr.strip() if e.stderr else str(e)
        raise ExtensionError(f"Git command failed for revision {revision}: {error_msg}")
    except subprocess.TimeoutExpired:
        raise ExtensionError(f"Git command timed out for revision {revision}")
    except ExtensionError:
        raise
    except Exception as e:
        raise ExtensionError(f"Unexpected error getting latest stable release for revision {revision}: {e}")


def setup_version_context_substitutions(app: Sphinx, config: Config) -> None:
    if not hasattr(config, 'myst_substitutions'):
        raise ExtensionError("myst_substitutions not found in config")

    # Get the repository root directory from the original checkout (app.confdir).
    # This is used for git operations that work across all versions (e.g., listing tags).
    repo_root = os.path.abspath(os.path.join(app.confdir, '..', '..'))
    if not os.path.exists(os.path.join(repo_root, '.git')):
        raise ExtensionError(f"Not a git repository: {repo_root}")

    # Get the repository root directory for the version being built (app.srcdir).
    # With sphinx-multiversion, this points to the temporary directory containing version-specific files.
    current_repo_root = os.path.abspath(os.path.join(app.srcdir, '..', '..'))

    # Get current version being built (from sphinx-multiversion or default to master).
    current_version = os.environ.get('SPHINX_MULTIVERSION_NAME', 'master')

    # Update myst_substitutions with version context.

    # The current branch/version being built.
    config.myst_substitutions['revision'] = current_version

    # Add latest stable release information.
    # Use the original checked-out repo root for git operations.
    latest_stable_release = get_latest_release_for_revision(current_version, repo_root)
    if latest_stable_release is not None:
        # The latest stable release (non-prerelease) for the revision
        config.myst_substitutions['latestStableRelease'] = latest_stable_release

        stripped_latest_stable_release = latest_stable_release.lstrip('v')
        # The latest stable release tag without the 'v' prefix
        config.myst_substitutions['latestStableVersion'] = stripped_latest_stable_release
        # The latest stable ScyllaDB Operator image tag
        config.myst_substitutions['latestStableImageTag'] = stripped_latest_stable_release

    # Add version information from config.yaml.
    # Use the version-specific repo root to read the correct config file.
    config_versions = get_versions_from_config(current_repo_root)
    config.myst_substitutions['scyllaDBImageTag'] = config_versions['scyllaDBVersion']
    config.myst_substitutions['agentVersion'] = config_versions['scyllaDBManagerAgentVersion']


def setup(app: Sphinx) -> dict[str, Any]:
    app.connect('config-inited', setup_version_context_substitutions)

    return {
        'version': '0.1',
        'parallel_read_safe': True,
        'parallel_write_safe': True,
    }
