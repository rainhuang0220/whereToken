#!/bin/sh
# Copy CI workflow files into .github/workflows (needs a token with the workflow scope).
set -e
cd "$(git rev-parse --show-toplevel)"
mkdir -p .github/workflows
cp ci/github-workflows/ci.yml .github/workflows/ci.yml
cp ci/github-workflows/release.yml .github/workflows/release.yml
cp ci/github-workflows/pages.yml .github/workflows/pages.yml
echo "wrote .github/workflows/ci.yml, release.yml and pages.yml"
echo "commit and push with a token that has the workflow scope"
