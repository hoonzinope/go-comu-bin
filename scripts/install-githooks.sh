#!/usr/bin/env sh

set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

mkdir -p "$repo_root/.git/hooks"
ln -sf ../../githooks/pre-commit "$repo_root/.git/hooks/pre-commit"

echo "Installed pre-commit hook -> .git/hooks/pre-commit"
