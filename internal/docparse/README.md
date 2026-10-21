# docparse sandbox image

`internal/docparse/pdf` runs poppler's `pdfinfo` and `pdftotext` on one untrusted PDF. Production code imports it only from `cmd/rzi-extract`. The RZI pipeline that calls the extractor is in [internal/upstream/rzi/README.md](../upstream/rzi/README.md). The host timer that starts the sandbox is in [deploy/README.md](../../deploy/README.md).

## Build

`Dockerfile.docparse` builds `ghcr.io/<owner>/kanarche-docparse`: `cmd/rzi-extract` plus poppler, run once per batch with no network. The publish workflow builds, scans, signs (cosign keyless, same identity as the main image) and pushes it next to the main image.

## Pins

| What | Pin |
|---|---|
| Base | `debian:trixie-slim@sha256:a29215f6a35e51e22adffa17f89e9d2ef06214e64a2bad10d765c46aea49f11f` (Debian 13.7) |
| Poppler | poppler-utils 25.03, unpinned from the trixie base (`25.03.0-5+deb13u4` at time of writing) |
| Go build stage | same `golang:1.26.10@sha256:...` as `Dockerfile` |

The base is pinned by digest. poppler-utils is not pinned, so a rebuild picks up Debian security revisions. The integration test holds `pdftotext -v` to the 25.x series at 25.03 or later. Trivy in the publish workflow is the CVE gate.

## NVD check (2026-10-11)

NVD keyword `poppler` and the Debian tracker for trixie. Open in trixie at this date:

- CVE-2026-107705, `Decrypt::revision6Hash` stack overflow. Reached by AES-256 encrypted PDFs before the gate refuses them. No Debian fix yet.
- CVE-2026-93311..93314, CVE-2026-102620, CVE-2026-102621, CVE-2026-93653: no DSA, postponed by Debian. Splash ones are not used by `pdfinfo`/`pdftotext`.

The sandbox flags below are the mitigation. Recheck on every pin bump.

## Image

- poppler-utils only, `--no-install-recommends`, apt lists removed.
- `USER 65534:65534`, entrypoint `/usr/local/bin/rzi-extract`.
- Writes only to `/out` and a tmpfs `/tmp`. It runs with a read-only root.
- `netprobe` exists only in the `probe` build target, used by tests. It is not in the published image (tested).
- The Debian base still contains a shell. Nothing in the run flags or entrypoint uses it.

## Run flags

```
--network none --read-only --tmpfs /tmp:size=16m,noexec --cap-drop ALL
--security-opt no-new-privileges --user 65534:65534 --memory 256m --memory-swap 256m
--cpus 0.5 --pids-limit 64 -v inbox:/in:ro -v outbox:/out
```

Per file the test kills the run at 20 s.

## Tests

`go test -tags docparse ./internal/docparse/integration/...` builds both targets. It runs each hostile input in its own container. Each run must end in a refusal record or a killed run, never `ok`.

- Inputs are generated in `hostile_test.go`. Nothing is committed or downloaded.
- `TestPositiveControls` runs the same flags on the Varna test excerpt laid out as a PDF. It also runs them on the committed Burgas workbook `internal/upstream/rzi/testdata/burgas-control.xls`, which the burgas package tests keep current. Both must give an ok record with the same blocks as the in-process extractor.
- `TestNetworkNone` starts a listener container on the default bridge. The control run must reach it. The sandboxed run must not (I1).

The xlsx inputs run through the Dobrich extractor and `internal/xlsx`. They exercise the zip, ratio, entry and XML depth caps and must come back as refusal records.
