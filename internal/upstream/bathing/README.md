# bathing — EEA bathing-water import (Sea layer)

Source: EEA WISE Bathing Water Directive data via the Discodata SQL API
(`sea.url`, plain GET, no auth). Licence CC BY 4.0; the credit is shown on the
map panel, the footer and `/licences`. OpenProject #668.

Three queries per import, each scoped by `countryCode='<sea.country>'`:

| Table (`[WISE_BWD].[latest]`) | What we keep |
|---|---|
| `spatial_ProtectedArea` | id, names (bg + Latin), zone (coastal / lake), lat, lon, profile link |
| `assessment_BathingWaterStatus` | annual class per season (2007 onward) |
| `assessment_MonitoringResult` | per-sample E. coli and intestinal enterococci, cfu/100 ml (2021 onward) |

`sea.country` is the only value spliced into the SQL; config validation and
`Fetch` both require two uppercase letters.

## Validation (`Build`)

- Retired sites are dropped, and with them their classes and samples.
- Zones other than coastal and lake are dropped (rivers would need their own limits).
- A site needs a well-formed id with the country prefix, real coordinates and a name.
- Profile links are kept only if they are absolute http(s) URLs.
- Censored results (`limitOfDetectionValue`) keep the number and set `*_below_detection`.
- Duplicate keys, unknown class labels and unknown value statuses are skipped and counted.
- No surviving site is an error: an empty answer never wipes the stored layer.

The fetch is bounded by `sea.max_payload_bytes`, `sea.request_timeout` and
`sea.max_rows`; a page as long as `max_rows` is an error, since rows may be
missing behind it. Redirects off the configured origin are refused.

## Cadence

The EEA publishes these tables once a year (season N appears around autumn of
N+1); BG samples total about 800 KB. The import runs inside `airbg serve` (and
`airbg collect`) every `sea.refresh_interval` (168h). The first wait counts
from the newest `bathing_import` row, so restarts and deploys do not re-fetch;
a fresh database imports immediately. A failed run retries after an hour and
leaves the stored set untouched.

`airbg import-sea` runs one import immediately, for a forced refresh.

## Datahub supplement (2025 classes)

Discodata stops at season 2024. The EEA publishes the 2025 annual class only as
an Excel workbook on the Datahub (released 2 Jun 2026). A hardened xlsx reader
built on stdlib runs offline in the dev command `extract-bathing-datahub`. It
validates the pinned workbook (sha256 checked), parses BG data, and writes a
reviewed snapshot (`internal/upstream/bathing/datahub/bg.json`) that is
committed and embedded in the binary.

At runtime, `Build` merges snapshot classes into Discodata for `(site, season)`
keys Discodata lacks, applying guards: Discodata wins on overlap, the snapshot
fills new seasons only, and disagreement on shared keys or low site coverage
refuses the whole supplement (logged). The snapshot edition trigger: Loop
imports at once when the embedded edition differs from stored, not waiting for
the interval.

New edition detection (weekly, after successful import): HEAD probes the next
edition URL (year bumped by one). 200 sets gauge `airbg_sea_datahub_newer_edition`
to 1, 404 to 0, other statuses return an error and leave it alone. Yearly
runbook: bump URL and sha in `airbg.yaml`, run extract command, review diff,
open a PR.
