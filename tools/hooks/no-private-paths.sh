#!/bin/sh
# Blocks forced adds of gitignored private paths: CLAUDE.md, docs/backlog, docs/superpowers, .superpowers.
set -eu
for f in "$@"; do
  case "$f" in
    CLAUDE.md|*/CLAUDE.md|docs/backlog/*|docs/superpowers/*|.superpowers/*)
      echo "private path staged: $f" >&2
      exit 1 ;;
  esac
done
