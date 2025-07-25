from typing import Any
from sphinx.application import Sphinx
from sphinx.config import Config

def config_inited(app: Sphinx, config: Config) -> None:
    substitutions_by_version = getattr(config, 'myst_multiversion_substitutions', None)
    if not substitutions_by_version:
        return

    current_version = config.myst_multiversion_substitutions_default_version
    if config.smv_current_version is not None and config.smv_current_version != '':
        current_version = config.smv_current_version
    if not current_version:
        raise AttributeError("'smv_current_version' is not set or is empty in config.")

    try:
        substitutions = substitutions_by_version[current_version]
    except KeyError:
        raise KeyError(f"Current version '{current_version}' not found in myst_multiversion_substitutions.")

    config.myst_substitutions.update(substitutions)

def setup(app: Sphinx) -> dict[str, Any]:
    app.add_config_value('myst_multiversion_substitutions_default_version', 'master', 'html')
    app.add_config_value('myst_multiversion_substitutions', {}, 'html')
    app.connect("config-inited", config_inited)

    return {
        'version': '0.1',
        'parallel_read_safe': True,
        'parallel_write_safe': True,
    }
