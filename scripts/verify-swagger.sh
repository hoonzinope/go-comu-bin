#!/usr/bin/env sh

set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

cd "$repo_root"

make swagger

if ! git diff --quiet -- docs/swagger; then
  echo "Swagger output is out of date. Run 'make swagger' and commit docs/swagger changes."
  git diff -- docs/swagger
  exit 1
fi
