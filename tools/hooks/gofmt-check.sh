#!/bin/sh
# Fails and lists any staged .go file that gofmt would change. Never rewrites.
set -eu
bad=$(gofmt -l "$@")
if [ -n "$bad" ]; then
  echo "gofmt needed on:" >&2
  echo "$bad" >&2
  exit 1
fi
