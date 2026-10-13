# RZI test fixtures

Short excerpts for parser tests. The full files are not committed: the RZI
pages say "All rights reserved" (decision D3).

## Origin

- `varna-2026-excerpt.txt`: `pdftotext -layout -enc UTF-8` of
  `https://www.rzi-varna.com/health/mvodi-2026.pdf` (fetched 2026-10-06,
  Last-Modified 2026-09-23). Kept lines: page 1 (method header, lines 1-25),
  page 3 (point 03002 block, lines 56-87), and the block for point 03011
  that holds the `4 124` / `34 659` row (lines 341-385).
- `varna-2025-excerpt.txt`: same tool on `mvodi-2025.pdf`, lines 1-36 (point 03001 block).
- `pages/varna-section.html`, `pages/burgas-section.html`,
  `pages/dobrich-section.html`: discovery pages trimmed to their links
  (fetched 2026-10-11). Dobrich keeps its internal `myconnect` link as served.

## Regenerate

    pdftotext -layout -enc UTF-8 mvodi-2026.pdf v26.txt
    sed -n '1,25p;56,87p;341,385p' v26.txt > varna-2026-excerpt.txt

Line numbers shift if the source file changes. Check the `4 124` row is still in the block.

## Cut

- Other Varna points (03004-03023 and the 2025 remainder).
- The footer and page-number lines (`стр.N`) between excerpts. They are not kept.
- Burgas and Dobrich data files. Their synthetic equivalents are built by test helpers.
  The Dobrich workbook (one sheet per point, label and coordinates above a header row) is
  rebuilt by `dobrich/testhelper_test.go` with `xlsxtest`. To check a downloaded copy, set
  `RZI_DOBRICH_FILE` (and optionally `RZI_DOBRICH_SITES`, a JSON list of EEA sites) and run
  `go test -run TestRealFile -v ./internal/upstream/rzi/dobrich/`. It logs counts only.
