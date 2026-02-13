"""
Copyright (C) 2026 ScyllaDB

Sphinx extension to extend myst_substitutions with version context in build time.
"""

import os
import re
import subprocess
from functools import reduce
from typing import Any, Optional

import yaml
from sphinx.application import Sphinx
from sphinx.config import Config
from sphinx.errors import ExtensionError

DEFAULT_BRANCH = 'master'


def read_yaml_file(file_path: str) -> dict[str, Any]:
    """
    Read and parse a YAML file.

    Args:
        file_path: Path to the YAML file.

    Returns:
        Parsed YAML data as a dictionary.

    Raises:
        ExtensionError: If the file doesn't exist or parsing fails.
    """
    if not os.path.exists(file_path):
        raise ExtensionError(f"File not found: {file_path}")

    try:
        with open(file_path, 'r') as f:
            return yaml.safe_load(f)
    except yaml.YAMLError as e:
        raise ExtensionError(f"Failed to parse YAML file {file_path}: {e}")
    except Exception as e:
        raise ExtensionError(f"Error reading YAML file {file_path}: {e}")


def get_nested_value(data: dict, key_path: str) -> Optional[str]:
    """
    Get a value from a nested dictionary using dot notation (e.g., 'operator.scyllaDBVersion').

    Args:
        data: The dictionary to query.
        key_path: Dot-separated path to the value (e.g., 'operator.scyllaDBVersion').

    Returns:
        The string value at the specified path, or None if not found.
        Non-string values (e.g., numbers parsed by YAML) are converted to strings.
    """
    try:
        result = reduce(lambda d, key: d.get(key) if isinstance(d, dict) else None, key_path.split('.'), data)
        if result is not None:
            return str(result)

        return None
    except (KeyError, TypeError):
        return None


def get_versions_from_config(repo_root: str) -> dict[str, str]:
    """
    Read version information from assets/config/config.yaml.

    This function gracefully handles missing files and fields, but will raise an error
    if the file exists but is invalid (unparseable YAML).

    Note: If substitution variables are not set here, references to them in documentation
    will cause build failures. Ensure the config file contains the required fields
    for the documentation being built.

    Args:
        repo_root: Path to the repository root directory. Should point to the version-specific
                   directory (sphinx-multiversion temporary directory when building multi-version docs).

    Returns:
        Dictionary with versions extracted from config.yaml.

    Raises:
        ExtensionError: If the config file exists but cannot be parsed.
    """
    config_path = os.path.join(repo_root, 'assets', 'config', 'config.yaml')

    # If config file doesn't exist, return empty dict (graceful handling for older branches)
    if not os.path.exists(config_path):
        return {}

    # If file exists but can't be parsed, raise an error
    config_data = read_yaml_file(config_path)

    # Strip digest if present (e.g., "3.8.0@sha256:..." -> "3.8.0")
    def strip_digest(version: str) -> str:
        return version.split('@')[0] if '@' in version else version

    result = {}

    # Try to extract ScyllaDB version
    scylla_db_version = get_nested_value(config_data, 'operator.scyllaDBVersion')
    if scylla_db_version is not None:
        result['scyllaDBImageTag'] = strip_digest(scylla_db_version)

    # Try to extract ScyllaDB Manager Agent version
    scylla_db_manager_agent_version = get_nested_value(config_data, 'operator.scyllaDBManagerAgentVersion')
    if scylla_db_manager_agent_version is not None:
        result['agentVersion'] = strip_digest(scylla_db_manager_agent_version)

    return result

def format_version_range(min_version: str, max_version: str) -> str:
    """
    Format a version range from min and max versions.

    Args:
        min_version: Minimum version string.
        max_version: Maximum version string.

    Returns:
        Version range string. If min and max are the same, returns just the version.
        Otherwise, returns the range in the format "min_version - max_version".
    """
    if min_version == max_version:
        return min_version
    else:
        return f"{min_version} - {max_version}"


def extract_version_range(
    data: dict,
    min_key: str,
    max_key: str,
) -> Optional[str]:
    """
    Extract and format a version range from nested dictionary data.

    Both min and max values must be present together. If either is missing,
    returns None.

    Args:
        data: Dictionary containing the version data.
        min_key: Dot-separated path to minimum version (e.g., 'operator.minOpenShiftVersion').
        max_key: Dot-separated path to maximum version (e.g., 'operator.maxOpenShiftVersion').

    Returns:
        Formatted version range string, or None if either min or max is missing.
    """
    min_version = get_nested_value(data, min_key)
    max_version = get_nested_value(data, max_key)

    # Both values must be present together
    if min_version is None or max_version is None:
        return None

    return format_version_range(min_version, max_version)


def get_versions_from_metadata(repo_root: str) -> dict[str, str]:
    """
    Read version information from assets/metadata/metadata.yaml.

    This function gracefully handles missing files and fields, but will raise an error
    if the file exists but is invalid (unparseable YAML). Version ranges are only added
    to the result if both min and max values are present together.

    Note: If substitution variables are not set here, references to them in documentation
    will cause build failures. Ensure the metadata file contains the required fields
    for the documentation being built.

    Args:
        repo_root: Path to the repository root directory. Should point to the version-specific
                   directory (sphinx-multiversion temporary directory when building multi-version docs).

    Returns:
        Dictionary with versions extracted from metadata.yaml.

    Raises:
        ExtensionError: If the metadata file exists but cannot be parsed.
    """
    metadata_path = os.path.join(repo_root, 'assets', 'metadata', 'metadata.yaml')

    # If metadata file doesn't exist, return empty dict (graceful handling for older branches)
    if not os.path.exists(metadata_path):
        return {}

    # If file exists but can't be parsed, raise an error
    metadata_data = read_yaml_file(metadata_path)

    result = {}

    # Try to extract OpenShift version range (both min and max must be present)
    openshift_range = extract_version_range(
        metadata_data,
        'operator.minOpenShiftVersion',
        'operator.maxOpenShiftVersion',
    )
    if openshift_range is not None:
        result['supportedOpenShiftVersionRange'] = openshift_range

    return result


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
        if revision != DEFAULT_BRANCH:
            match = re.match(r'^v(\d+\.\d+)$', revision)
            if not match:
                raise ExtensionError(
                    f"Could not parse version from revision '{revision}'. Expected format: '{DEFAULT_BRANCH}' or vX.Y")

            major_minor = match.group(1)
            matching_tags = [tag for tag in stable_tags if re.match(rf'^v{re.escape(major_minor)}\.', tag)]

            if matching_tags:
                return matching_tags[0]

        # For default branch or branches with no stable releases, return the latest stable release.
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

    # Get current version being built (from sphinx-multiversion or default).
    current_version = os.environ.get('SPHINX_MULTIVERSION_NAME', DEFAULT_BRANCH)

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
    config.myst_substitutions.update(get_versions_from_config(current_repo_root))

    # Add version information from metadata.yaml.
    # Use the version-specific repo root to read the correct metadata file.
    config.myst_substitutions.update(get_versions_from_metadata(current_repo_root))


def setup(app: Sphinx) -> dict[str, Any]:
    app.connect('config-inited', setup_version_context_substitutions)

    return {
        'version': '0.1',
        'parallel_read_safe': True,
        'parallel_write_safe': True,
    }
