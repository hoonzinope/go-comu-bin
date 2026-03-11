#!/usr/bin/env sh

set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

cd "$repo_root"

before_diff=$(git diff -- docs/swagger)

make swagger

after_diff=$(git diff -- docs/swagger)

if [ "$before_diff" != "$after_diff" ]; then
  echo "Swagger output is out of date. Run 'make swagger' and review docs/swagger changes."
  git diff -- docs/swagger
  exit 1
fi
