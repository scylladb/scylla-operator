# Contributing to Scylla Operator

## Prerequisites

To develop on scylla-operator, your environment must have the following:

1. [Go 1.13](https://golang.org/dl/)
    * Make sure [GOPATH](https://github.com/golang/go/wiki/SettingGOPATH) is set to `GOPATH=$HOME/go`.
2. [Kustomize v3.1.0](https://github.com/kubernetes-sigs/kustomize/releases/tag/v3.1.0)
3. [kubebuilder v2.3.1](https://github.com/kubernetes-sigs/kubebuilder/releases/tag/v2.3.1)
5. [Docker](https://docs.docker.com/install/)
6. [goreleaser](https://goreleaser.com/)
7. Git client installed
8. Github account

To install all dependencies (Go, kustomize, kubebuilder, dep), simply run:
```bash
./install-dependencies.sh
```

## Initial Setup

### Create a Fork

From your browser navigate to [http://github.com/scylladb/scylla-operator](http://github.com/scylladb/scylla-operator) and click the "Fork" button.

### Clone Your Fork

Open a console window and do the following:

```bash
# Create the scylla operator repo path
mkdir -p $GOPATH/src/github.com/scylladb

# Navigate to the local repo path and clone your fork
cd $GOPATH/src/github.com/scylladb

# Clone your fork, where <user> is your GitHub account name
git clone https://github.com/<user>/scylla-operator.git
```

### Add Upstream Remote

First you will need to add the upstream remote to your local git:
```bash
# Add 'upstream' to the list of remotes
git remote add upstream https://github.com/scylladb/scylla-operator.git

# Verify the remote was added
git remote -v
```
Now you should have at least `origin` and `upstream` remotes. You can also add other remotes to collaborate with other contributors.

## Development

To add a feature or to make a bug fix, you will need to create a branch in your fork and then submit a pull request (PR) from the branch.

### Building the project

You can build the project using the Makefile commands:
* Open the Makefile and change the `IMG` environment variable to a repository you have access to.
* Run `make docker-push` and wait for the image to be built and uploaded in your repo.

### Create a Branch

From a console, create a new branch based on your fork and start working on it:

```bash
# Ensure all your remotes are up to date with the latest
git fetch --all

# Create a new branch that is based off upstream master.  Give it a simple, but descriptive name.
# Generally it will be two to three words separated by dashes and without numbers.
git checkout -b feature-name upstream/master
```

Now you are ready to make the changes and commit to your branch.

### Updating Your Fork

During the development lifecycle, you will need to keep up-to-date with the latest upstream master. As others on the team push changes, you will need to `rebase` your commits on top of the latest. This avoids unnecessary merge commits and keeps the commit history clean.

Whenever you need to update your local repository, you never want to merge. You **always** will rebase. Otherwise you will end up with merge commits in the git history. If you have any modified files, you will first have to stash them (`git stash save -u "<some description>"`).

```bash
git fetch --all
git rebase upstream/master
```

Rebasing is a very powerful feature of Git. You need to understand how it works or else you will risk losing your work. Read about it in the [Git documentation](https://git-scm.com/docs/git-rebase), it will be well worth it. In a nutshell, rebasing does the following:
- "Unwinds" your local commits. Your local commits are removed temporarily from the history.
- The latest changes from upstream are added to the history
- Your local commits are re-applied one by one
- If there are merge conflicts, you will be prompted to fix them before continuing. Read the output closely. It will tell you how to complete the rebase.
- When done rebasing, you will see all of your commits in the history.

## Submitting a Pull Request

Once you have implemented the feature or bug fix in your branch, you will open a PR to the upstream repo. Before opening the PR ensure you have added unit tests, are passing the integration tests, cleaned your commit history, and have rebased on the latest upstream.

In order to open a pull request (PR) it is required to be up to date with the latest changes upstream. If other commits are pushed upstream before your PR is merged, you will also need to rebase again before it will be merged.

### Commit History

To prepare your branch to open a PR, you will need to have the minimal number of logical commits so we can maintain
a clean commit history. Most commonly a PR will include a single commit where all changes are squashed, although
sometimes there will be multiple logical commits.

```bash
# Inspect your commit history to determine if you need to squash commits
git log

# Rebase the commits and edit, squash, or even reorder them as you determine will keep the history clean.
# In this example, the last 5 commits will be opened in the git rebase tool.
git rebase -i HEAD~5
```

Once your commit history is clean, ensure you have based on the [latest upstream](#updating-your-fork) before you open the PR.

### Commit messages

Please make the first line of your commit message a summary of the change that a user (not a developer) of Operator would like to read, 
and prefix it with the most relevant directory of the change followed by a colon. 
The changelog gets made by looking at just these first lines so make it good!

If you have more to say about the commit, then enter a blank line and carry on the description. 
Remember to say why the change was needed - the commit itself shows what was changed.

Writing more is better than less. Comparing the behaviour before the change to that after the change is very useful. 
Imagine you are writing to yourself in 12 months time when you've forgotten everything about what you just did, and you need to get up to speed quickly.

If the change fixes an issue then write Fixes #1234 in the commit message. 
This can be on the subject line if it will fit. If you don't want to close the associated issue just put #1234 and the change will get linked into the issue.

Here is an example of a short commit message:

```
sidecar: log on reconcile loop - fixes #1234
```

And here is an example of a longer one:
```

api: now supports host networking (#1234)

The operator CRD now has a "network" property that can be used to
select host networking as well as setting the apropriate DNS policy.

Fixes #1234
```

### Submitting

Go to the [Scylla Operator github](https://www.github.com/scylladb/scylla-operator) to open the PR. If you have pushed recently, you should see an obvious link to open the PR. If you have not pushed recently, go to the Pull Request tab and select your fork and branch for the PR.

After the PR is open, you can make changes simply by pushing new commits. Your PR will track the changes in your fork and update automatically.
