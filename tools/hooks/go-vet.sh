#!/bin/sh
# Runs go vet over the module with default build tags.
set -eu
exec go vet ./...
