#!/bin/sh
# Same rule as TestCopyHasNoSemicolonsOrDashes: no ';', em or en dash in i18n JSON.
# A digit-to-digit en dash (numeric range) is allowed.
set -eu
export LC_ALL=C
en=$(printf '\342\200\223')
em=$(printf '\342\200\224')
rc=0
for f in "$@"; do
  hits=$(sed -E "s/[0-9]${en}[0-9]//g" "$f" | grep -n ";\|${en}\|${em}" || true)
  if [ -n "$hits" ]; then
    echo "$f: ';', em dash or en dash in copy:" >&2
    echo "$hits" >&2
    rc=1
  fi
done
exit $rc
