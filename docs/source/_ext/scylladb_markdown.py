import os
from recommonmark.transform import AutoStructify

class BaseParser:
    def __init__(self, app):
        self.app = app

    def setup(self):
        raise NotImplementedError

class RecommonmarkParser(BaseParser):
    def setup(self):
        try:
            self.app.setup_extension('recommonmark')
            self.app.setup_extension('sphinx_markdown_tables')

            self.app.add_config_value('recommonmark_config', {
                'enable_eval_rst': True,
                'enable_auto_toc_tree': False,
            }, True)
            self.app.add_transform(AutoStructify)
        except ImportError:
            raise RuntimeError("recommonmark and sphinx_markdown_tables are not installed!")

class MystParser(BaseParser):
    def setup(self):
        try:
            self.app.setup_extension('myst_parser')
            self.app.config.myst_enable_extensions = ["colon_fence"]

            # TODO: https://github.com/scylladb/scylla-operator/issues/1421
            # TODO: https://github.com/scylladb/scylla-operator/issues/1422
            self.app.config.suppress_warnings = ["myst.xref_missing", "myst.header"]

        except ImportError:
            raise RuntimeError("myst-parser is not installed")

def select_parser(app, config):
    if not config.scylladb_markdown_enable:
        return

    current_version = os.environ.get('SPHINX_MULTIVERSION_NAME', 'stable')

    config.source_suffix = {
        '.rst': 'restructuredtext',
        '.md': 'markdown',
    }

    if current_version in config.scylladb_markdown_recommonmark_versions:
        parser = RecommonmarkParser(app)
    else:
        parser = MystParser(app)

    parser.setup()

def setup(app):
    app.add_config_value('scylladb_markdown_enable', True, 'env')
    app.add_config_value('scylladb_markdown_recommonmark_versions', [], 'env')
    app.connect('config-inited', select_parser)

    return {
        "version": "0.1",
        "parallel_read_safe": True,
        "parallel_write_safe": True,
    }
