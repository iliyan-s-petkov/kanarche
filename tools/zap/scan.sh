#!/usr/bin/env bash
# ZAP passive baseline against an ephemeral local instance of the app.
# The target is always the instance started here. See CONTRIBUTING.md.
#
# Env: ZAP_IMAGE (required, pinned by digest), ZAP_HOST (how the container
# reaches the host, default 127.0.0.1 with host networking), ZAP_NETWORK_ARGS.
set -euo pipefail

: "${ZAP_IMAGE:?set ZAP_IMAGE to the pinned zaproxy image}"
repo=$(cd "$(dirname "$0")/../.." && pwd)
work=$(mktemp -d)
trap 'touch "$work/stop"; wait "${app_pid:-}" 2>/dev/null || true; rm -rf "$work" 2>/dev/null || true' EXIT

# The app: the e2e stack (Postgres container, migrations, fixtures), no
# collectors, geocoder stubbed. TestServeForZAP serves until $work/stop exists.
(cd "$repo" && KANARCHE_ZAP_URL_FILE="$work/url" KANARCHE_ZAP_STOP_FILE="$work/stop" \
  go test -count=1 -tags e2e -run '^TestServeForZAP$' -timeout 25m -v ./internal/e2e/ \
  >"$work/app.log" 2>&1) &
app_pid=$!

for _ in $(seq 1 180); do
  [ -s "$work/url" ] && break
  kill -0 "$app_pid" 2>/dev/null || { tail -40 "$work/app.log"; echo "app exited before it was ready"; exit 1; }
  sleep 1
done
[ -s "$work/url" ] || { tail -40 "$work/app.log"; echo "app never became ready"; exit 1; }

port=$(sed -E 's#.*:([0-9]+)$#\1#' "$work/url")
host=${ZAP_HOST:-127.0.0.1}
target="http://$host:$port"
echo "ZAP target: $target (local ephemeral instance)"

# The only hosts ZAP may contact are the target. Nothing else is configured.
mkdir -p "$work/wrk"
cp "$repo/.zap/rules.tsv" "$repo/tools/zap/hook.py" "$work/wrk/"
chmod -R a+rwX "$work/wrk"

paths="/ /en/ /about-the-data /en/about-the-data /about /area/sofia /en/area/sofia /areas /build-a-sensor /air-pollution
/api/v1/meta /api/v1/overview /api/v1/areas /api/v1/area/sofia/sensors /api/v1/visitors /api/v1/scales /api/v1/hexes /api/v1/boundaries
/robots.txt /sitemap.xml /nope-does-not-exist"

set +e
# shellcheck disable=SC2086
docker run --rm ${ZAP_NETWORK_ARGS---network host} \
  -e ZAP_PATHS="$paths" \
  -v "$work/wrk:/zap/wrk:rw" \
  "$ZAP_IMAGE" zap-baseline.py -t "$target" -c rules.tsv --hook=/zap/wrk/hook.py -m 2 -T 8 -J zap.json 2>&1 \
  | tee "$work/zap.log" | grep -vE '^\s*$' | tail -n 120
status=${PIPESTATUS[0]}
set -e
# Short per-alert summary for the log. The full report is never uploaded.
if [ -s "$work/wrk/zap.json" ]; then
  python3 -I - "$work/wrk/zap.json" <<'PY'
import json, sys
for site in json.load(open(sys.argv[1]))["site"]:
    for a in site["alerts"]:
        if a["riskcode"] == "0":
            continue
        print("ALERT", a["pluginid"], a["name"], "risk", a["riskdesc"], "count", a["count"])
        for i in a["instances"][:2]:
            print("   ", i["uri"], "|", i.get("param", ""), "|", i.get("evidence", "")[:120].replace("\n", " "), "|", i.get("otherinfo", "")[:160].replace("\n", " "))
PY
fi
echo "zap-baseline exit code: $status"
exit "$status"
