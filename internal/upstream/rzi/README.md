# RZI bathing samples

This package ingests bathing-water sample documents from the regional health inspectorates (RZI). Varna, Burgas and Dobrich publish one document per season on their section pages. The package fetches them, hands them to the sandboxed extractor, and stores the samples that pass validation.

The extractor is a separate binary, see [cmd/rzi-extract](../../../cmd/rzi-extract/). Its container image and run flags are in [internal/docparse/README.md](../../docparse/README.md). The host timer and mounts are in [deploy/README.md](../../../deploy/README.md).

## Pipeline

1. Discovery (`discover.go`). The fetcher reads the source's section page and picks the one link whose path matches the current year. Links are read, not followed, and must be on the source's fixed host list.
2. Fetch (`fetch.go`). The document is downloaded and its magic bytes are checked. A new document is written to `inbox/<sha256>.<ext>`. The ETag and Last-Modified values from the last fetch are sent with the next request.
3. Extract. `cmd/rzi-extract` runs in the no-network sandbox over the inbox. It writes one record per document to `outbox/<sha256>.json`.
4. Ingest (`ingest.go`). The Ingester decodes each record and validates it (`validate.go`). Sites are matched to EEA bathing sites in the store (`sitemap.go`). Each row is also compared with the last ok file of the same source.
5. Store and move. The verdict is stored on a file row. The record moves to `done/`, `review/` or `bad/` under the outbox. The inbox copy is removed only when the document is accepted.
6. Approve (`approve.go`). A `needs_review` file stores no samples until an operator runs `airbg rzi-approve <sha256>`. Only the rows that passed validation are published. Flagged rows stay out.

## Sources

- `varna`: PDF text from poppler, parsed in `varna/`.
- `burgas`: XLS (BIFF) workbook, parsed in `burgas/`.
- `dobrich`: XLSX. Discovery and fetch are in place. The extractor is not built yet, so xlsx inputs are refused as `unsupported format`. Dobrich XLSX: see OP 707.

## Configuration

Keys live under `sea.rzi` in `airbg.yaml`.

- `enabled`: when true, `serve` runs the collector.
- `inbox_dir`, `outbox_dir`: the two directories shared with the extractor.
- `in_season_interval`, `off_season_interval`: the wait between passes.
- `max_file_bytes`: size cap for one downloaded document.
- `request_timeout`: timeout for one HTTP request.
- `varna.page_url`, `burgas.page_url`, `dobrich.page_url`: the section pages. The allowed hosts are fixed in code, not in config.

## Season

`SeasonOpen` is true from 15 May to 15 October inclusive, by the calendar date in Sofia. `Interval` returns `in_season_interval` while the season is open and `off_season_interval` otherwise. The validator uses the same months. The first good date in a file fixes its season to the current or the previous year.

## Failures

- A fetch error for one source is logged. The other sources still run.
- A store or disk error stops the ingest pass. The next pass runs after at most one hour.
- A record that fails strict decoding is moved to `bad/`.
