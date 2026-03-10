#!/usr/bin/env sh

set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

cd "$repo_root"

before_status=$(git status --porcelain -- docs/swagger)

make swagger

after_status=$(git status --porcelain -- docs/swagger)

if [ "$before_status" != "$after_status" ]; then
  echo "Swagger output is out of date. Run 'make swagger' and review docs/swagger changes."
  git diff -- docs/swagger
  exit 1
fi
