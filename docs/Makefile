# You can set these variables from the command line.
UV ?= uv
SPHINXBUILD ?= $(UV) run sphinx-build
SPHINXOPTS ?= -j auto
PAPER ?=
BUILDDIR ?= _build
SOURCEDIR ?= source
OS ?=


# Internal variables.
paperopt_a4 := -D latex_paper_size=a4
paperopt_letter := -D latex_paper_size=letter

ifdef $(PAPER)
paperopt :=$(paperopt_$(PAPER))
else
paperopt :=
endif

ALLSPHINXOPTS ?= -d $(BUILDDIR)/doctrees $(paperopt) $(SPHINXOPTS) $(SOURCEDIR) -W
# the i18n builder cannot share the environment and doctrees with the others
I18NSPHINXOPTS ?= $(paperopt) $(SPHINXOPTS) $(SOURCEDIR)

.PHONY: all
all: dirhtml

clean:
	rm -rf '$(BUILDDIR)'/*
.PHONY: clean

preview:
	$(UV) run sphinx-autobuild -b=dirhtml $(ALLSPHINXOPTS) $(BUILDDIR)/dirhtml --port=5500 --host=0.0.0.0
.PHONY: preview

dirhtml:
	$(SPHINXBUILD) -b dirhtml $(ALLSPHINXOPTS) $(BUILDDIR)/dirhtml
	@echo
	@echo "Build finished. The HTML pages are in $(BUILDDIR)/dirhtml."
.PHONY: dirhtml

singlehtml:
	$(SPHINXBUILD) -b singlehtml $(ALLSPHINXOPTS) $(BUILDDIR)/singlehtml
	@echo
	@echo "Build finished. The HTML page is in $(BUILDDIR)/singlehtml."
.PHONY: singlehtml

epub:
	$(SPHINXBUILD) -b epub $(ALLSPHINXOPTS) $(BUILDDIR)/epub
	@echo
	@echo "Build finished. The epub file is in $(BUILDDIR)/epub."
.PHONY: epub

epub3:
	$(SPHINXBUILD) -b epub3 $(ALLSPHINXOPTS) $(BUILDDIR)/epub3
	@echo
	@echo "Build finished. The epub3 file is in $(BUILDDIR)/epub3."
.PHONY: epub3

dummy:
	$(SPHINXBUILD) -b dummy $(ALLSPHINXOPTS) $(BUILDDIR)/dummy
	@echo
	@echo "Build finished. Dummy builder generates no files."
.PHONY: dummy

linkcheck:
	$(SPHINXBUILD) -b linkcheck $(SOURCEDIR) $(BUILDDIR)/linkcheck
.PHONY: linkcheck

multiversion:
	@mkdir -p $(HOME)/.cache/pypoetry/virtualenvs
	$(UV) run sphinx-multiversion $(SOURCEDIR) $(BUILDDIR)/dirhtml
	@echo
	@echo "Build finished. The HTML pages are in $(BUILDDIR)/dirhtml."
.PHONY: multiversion

multiversionpreview:
	$(UV) run python3 -m http.server 5500 --directory $(BUILDDIR)/dirhtml
.PHONY: multiversionpreview

redirects:
	$(UV) run redirects-cli fromfile --yaml-file ./_utils/redirects.yaml --output-dir $(BUILDDIR)/dirhtml
	@echo
	@echo "Build finished. The HTML pages are in $(BUILDDIR)/dirhtml."
.PHONY: redirects

setup:
	$(UV) sync --locked
.PHONY: setup

update:
	$(UV) sync --upgrade
.PHONY: update

verify-uv:
	$(UV) lock --check
.PHONY: verify-uv

verify: verify-uv linkcheck
.PHONY: verify
