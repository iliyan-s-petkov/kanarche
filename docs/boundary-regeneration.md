# Regenerating the boundary data

`data/boundaries/` holds four GeoJSON files that `airbg import-areas` loads into
the `area` table. They are treated as a **one-time artefact**, not a build
output: no script regenerates them, and nothing in CI rebuilds them. This
document is what makes them reproducible anyway.

Read it before regenerating anything. The transformation rules below — the slug
rule above all — live only here and in the committed files. A regeneration that
silently drops one produces a file that imports cleanly and breaks every inbound
URL.

`data/boundaries/README.md` covers what the files *are* (sources, licences,
counts, per-city geometry caveats). This covers how to *rebuild* them.

## Why regenerating is riskier than it looks

Slugs appear in URLs: `/oblast/{slug}`, `/area/{slug}`. Changing one breaks
every inbound link and every search-engine result for that page. Treat a shipped
slug as permanent.

`area.slug` is the `area` table's `PRIMARY KEY`, shared globally across every
`kind`, and `area.Import` upserts on it (`ON CONFLICT (slug) DO UPDATE`). A
Bulgarian oblast and its capital city share a name, so both transliterate to the
same string: importing `oblasti.geojson` then `cities.geojson` once rewrote 26 of
28 oblast rows into city rows — same slug, so the second import overwrote the
first rather than inserting. `SELECT count(*) FROM area WHERE kind = 'oblast'`
dropped from 28 to 2, and every per-file feature count stayed correct throughout.

## The slug rule

Slugs are transliterated from `name_bg` (not `name_en`) using the Bulgarian
**Streamlined System** — the official 2009 transliteration — applied strictly
per character:

| | | | | | | | | | |
|---|---|---|---|---|---|---|---|---|---|
| а `a` | б `b` | в `v` | г `g` | д `d` | е `e` | ж `zh` | з `z` | и `i` | й `y` |
| к `k` | л `l` | м `m` | н `n` | о `o` | п `p` | р `r` | с `s` | т `t` | у `u` |
| ф `f` | х `h` | ц `ts` | ч `ch` | ш `sh` | щ `sht` | ъ `a` | ь `y` | ю `yu` | я `ya` |

Then: lowercase; spaces and hyphens become `-`.

Two rules on top of the table:

1. **The word-final `ия` → `ia` exception in the transliteration law is not
   applied**, except София and София-град, which are pinned to `sofia` and
   `sofia-grad` (the plain `sofiya` was a typo; old URLs 301). `bulgaria.geojson`'s
   `bulgaria` slug looks like the exception but is not produced by this rule at
   all — that file comes from Natural Earth with a hand-set slug.
2. **Every oblast slug carries a uniform `-oblast` suffix**
   (`varna-oblast`, `sofia-grad-oblast`). Cities and Sofia districts keep the
   bare transliteration, because a city is what a reader searches for and the
   slug is what appears in its URL.

The suffix is uniform rather than applied only where a collision exists today.
A collision-driven rule breaks again the first time a new city is added whose
name matches an oblast that currently has no colliding city.

`name_en` is the conventional English exonym (`Sofia`, `Ruse`) and is display
copy only. Nothing keys on it.

**This rule is executable.** `internal/area/slug_rule_test.go` recomputes every
slug in all three OSM-derived files from that feature's own `name_bg` and fails
if the committed file disagrees, plus asserts the 80 slugs are globally
distinct. A regeneration that drops the `-oblast` suffix cannot be committed
green. Verified: all 79 OSM-derived slugs reproduce exactly from the table
above; there are no hand-edited exceptions to remember.

## Sources

### `bulgaria.geojson` — the national boundary

Regenerated 2026-09-27 as the union of `oblasti.geojson`'s 28 features and the
prior Natural Earth outline, differenced against the union of the five
neighbour files (see "Removing the neighbour overlap" below). Properties are
hand-set (`slug: bulgaria`, `iso_a2: BG`), unchanged by the regeneration.

**Why not Natural Earth any more:** the original file was Natural Earth 1:10m
Admin 0 – Countries, the feature with `ADM0_A3 = BGR`, geometry unmodified.
Its Black Sea coastline cut 0.25–0.45 km inside the true coast, which was
enough for `FilterByBoundary`'s `ST_Covers` to silently drop two live
sensors: 7669 (Pomorie, 27.636E 42.562N) and 32826 (Sinemorets, 27.986E
42.058N). Both are inside `burgas-oblast` in `oblasti.geojson`, which OSM
(and #614's ring-assembly fix) already gets right, so the country boundary is
now built from the same source instead of a separate, lower-resolution
release.

**Rebuild steps**, run against a scratch PostGIS container (any Postgres +
PostGIS the machine can reach; nothing is written back to it):

```sql
-- one row per oblast feature, geometry loaded via ST_GeomFromGeoJSON
CREATE TABLE oblast (id serial primary key, slug text, geom geometry(MultiPolygon, 4326));
-- ... INSERT one row per feature in oblasti.geojson ...

CREATE TABLE bulgaria_union AS
SELECT ST_MakeValid(ST_Union(ST_MakeValid(geom))) AS geom FROM oblast;

-- union in the prior Natural Earth outline too: OSM's admin boundary along
-- the Danube excludes some river/bank territory that Natural Earth includes,
-- and that strip does not overlap Romania's own polygon (checked directly
-- via ST_Intersection, not inferred from a centroid test), so it is safe to
-- keep rather than a regression to guard against
CREATE TABLE bulgaria_with_old_outline AS
SELECT ST_MakeValid(ST_Union(a.geom, b.geom)) AS geom
FROM bulgaria_union a, old_natural_earth_bulgaria b;
```

`ST_Union` over 28 independently-simplified oblast polygons leaves gaps where
two neighbours' shared border doesn't line up vertex-for-vertex — 414
interior rings in the 2026-09-27 run, largest 0.39 km², all under 1 km². Fill
any interior ring under 1 km² rather than leaving it as a hole:

```sql
-- for each part with interior rings, rebuild it keeping only rings >= 1 km²
SELECT ST_MakePolygon(ST_ExteriorRing(p), keep_rings)
FROM ... -- see internal/area/committed_boundaries_test.go's
         -- TestCountryBoundaryCoversEveryOblast for the check this feeds
```

After filling, the result is a `MultiPolygon` with 10 parts: one mainland part
(~111,001 km²) plus nine parts under 0.2 km² each that are **not** union
artifacts — they are already separate parts of `burgas-oblast`'s own OSM
geometry (small exclaves near the coast/border), confirmed by dumping that
one oblast's geometry alone before the union. Total area ~111,016 km²,
against Bulgaria's official ~110,994 km² — within simplification tolerance.
Export with `ST_AsGeoJSON(geom, 6)` to match the file's 6-decimal-place
coordinate precision.

Acceptance, beyond the standard checks below: every oblast's
`ST_PointOnSurface` must be `ST_Covers`ed by the rebuilt country polygon (a
seam or gap would fail a specific oblast, not just a total-area check), and
the Pomorie/Sinemorets points must now read `true`.

**Removing the neighbour overlap:** Bulgaria is now built from OSM (plus the
prior Natural Earth outline) while the five neighbours (`greece.geojson`,
`north-macedonia.geojson`, `romania.geojson`, `serbia.geojson`,
`turkey.geojson`) are still Natural Earth. Left alone, the two sources
disagree on the shared border by a thin strip (~144 km² with Greece down to
~1 km² with North Macedonia), and `FilterByBoundary`'s country tiebreak sorts
`BG` first alphabetically, so any sensor inside one of those strips would be
attributed to Bulgaria rather than its true neighbour. Rather than accept
that risk, the union above is differenced against the neighbours before
export:

```sql
CREATE TABLE neighbours_union AS
SELECT ST_MakeValid(ST_Union(ST_MakeValid(geom))) AS geom FROM neighbour;

CREATE TABLE bulgaria_final AS
SELECT ST_MakeValid(ST_Difference(a.geom, b.geom)) AS geom
FROM bulgaria_with_old_outline a, neighbours_union b;
```

This removes only the genuine overlap (~308 km² total, matching the five
figures above almost exactly) and leaves the Danube strip untouched, since
that strip does not actually intersect Romania's polygon. The result's
`ST_Intersection` area with each of the five neighbour files is under
0.01 km² (residual floating-point noise from rounding two independently
sourced datasets to 6 decimal places, not real overlap), asserted by
`internal/area/committed_boundaries_test.go`'s
`TestCountryBoundaryDoesNotOverlapNeighbours` on every run. Re-run the
`ST_Difference` once more after exporting at 6-decimal precision if the
rounding itself reintroduces a larger residual — it should not, since
`ST_Difference` at that point runs against the exact geometry of the
committed neighbour files rather than the pre-rounding union.

### `oblasti.geojson`, `cities.geojson`, `sofia-districts.geojson`

OpenStreetMap via the Overpass API (`overpass-api.de` or
`overpass.kumi.systems`), retrieved 2026-08-10. **Licence: ODbL 1.0** —
"© OpenStreetMap contributors" must appear in the site footer. That is a licence
obligation, not a courtesy.

The queries below are stated in the same terms `data/boundaries/README.md`
documents the data by. Bulgaria's OSM admin levels are **not uniform across
tiers**, which is the single most important thing to know before rerunning them.

**Oblasti — 28 features.** `boundary=administrative`, `admin_level=4`, inside
the `ISO3166-1=BG` area. One relation per oblast, matches exactly.

```overpassql
[out:json][timeout:180];
area["ISO3166-1"="BG"][admin_level=2]->.bg;
relation(area.bg)["boundary"="administrative"]["admin_level"="4"];
out geom;
```

**Cities — 27 features, not 28.** Sofia is the seat of both Sofia-grad and Sofia
Oblast, so 28 oblasti have only 27 distinct capitals. A 28th feature would mean
committing Sofia's polygon twice under two slugs.

Level differs per city and this is the part that cannot be automated blindly:

- 13 capitals have an `admin_level=8` city-proper boundary tagged `place=city`
  or `place=town`: Sofia, Ruse, Dobrich, Razgrad, Targovishte, Silistra, Shumen,
  Varna, Gabrovo, Veliko Tarnovo, Kyustendil, Burgas, Vidin.
- The other 14 have **no** settlement-level boundary in OSM, so their
  `admin_level=5` (`border_type=municipality`) boundary is used: Blagoevgrad,
  Vratsa, Kardzhali, Lovech, Montana, Pazardzhik, Pernik, Pleven, Plovdiv,
  Sliven, Smolyan, Haskovo, Stara Zagora, Yambol.

```overpassql
[out:json][timeout:180];
area["ISO3166-1"="BG"][admin_level=2]->.bg;
(
  relation(area.bg)["boundary"="administrative"]["admin_level"="8"]["name"="<capital>"];
  relation(area.bg)["boundary"="administrative"]["admin_level"="5"]["name"="<capital>"];
);
out geom;
```

Run per capital and take the `admin_level=8` result when one exists. If a rerun
finds an `admin_level=8` boundary for one of the 14, **that is a data
improvement, not a bug** — but it changes that city's polygon and therefore its
aggregate values, so make it a deliberate, separately reviewed commit and update
both the 13/14 split here and in `docs/known-limitations.md`.

**Sofia districts — 24 features.** `boundary=administrative`, `admin_level=6`,
inside the area named `Столична` (`admin_level=5`, the Stolichna municipality).
Not `admin_level=9`, which under `Столична` is a finer subdivision than the
district level.

```overpassql
[out:json][timeout:180];
area["name"="Столична"]["admin_level"="5"]->.sofia;
relation(area.sofia)["boundary"="administrative"]["admin_level"="6"];
out geom;
```

`out geom;` returns full outer-ring coordinates inline — no separate
`out body`/`out skel` pass is needed.

## Cleaning and simplification

Raw Overpass geometry is not importable as-is, and it is not yet a single
polygon either. `out geom;` on a relation returns one line geometry **per
member way**, not the assembled outer/inner ring: a typical oblast relation is
tagged together from dozens to hundreds of ways, each one a segment of the
boundary shared with a neighbouring relation. Handing those ways to PostGIS as
if each were already a ring — e.g. loading them individually and unioning the
result — produces a `MultiPolygon` with one sliver "part" per way: it parses,
it imports, `ST_IsValid` even holds, and its total area is a fraction of the
real place because only the ways happen to close on themselves that get
counted, while the rest trace the perimeter without ever enclosing anything.
This is what shipped for 21 of cities.geojson's 27 features (Plovdiv: 19 parts,
~6 points each, ~24 km² against a real ~102 km² municipality) — every
downstream check that only looked at parsed feature counts or `ST_IsValid`
passed, because both were true.

The member ways must be assembled into rings **before** cleaning. Run each
relation's collected member-way geometries through, in this order:

```sql
SELECT ST_CollectionExtract(
         ST_SimplifyPreserveTopology(
           ST_MakeValid(
             ST_BuildArea(
               ST_Collect(
                 ST_LineMerge(ST_Collect(member_way_geom))
               )
             )
           ),
           0.002),          -- roughly 200 m
         3)                 -- 3 = polygons only
       FROM relation_members
       GROUP BY relation_id
```

Each step earns its place:

- **`ST_Collect` + `ST_LineMerge` first.** Stitches the member ways back into
  contiguous closed rings. A relation's ways arrive in no particular order and
  a single ring is frequently split across several of them at each node where
  two neighbouring relations' boundaries meet; `ST_LineMerge` joins ways that
  share an endpoint into one linestring per ring.
- **`ST_BuildArea`** turns the merged, closed linestrings into the actual
  polygon(s) — outer rings become shells, rings nested inside them become
  holes. This is the step that was missing: without it, PostGIS has no way to
  know which of the many rings enclose area and which are holes, or that they
  belong to one polygon at all.
- **`ST_MakeValid` after assembly.** Several relations contain a spurious
  near-zero-length "outer" member way — an OSM digitisation artefact. Treated as
  its own ring it collapses into a degenerate line under simplification, turning
  the result into a `GeometryCollection` that `area.Import`'s `validateGeometry`
  rejects. Repairing before simplifying avoids hand-editing source rings.
- **`ST_SimplifyPreserveTopology`, not `ST_Simplify`.** Plain simplification can
  emit self-intersecting rings on its own.
- **`ST_CollectionExtract(..., 3)`** drops stray linestrings.

`internal/area/committed_boundaries_test.go`'s
`TestBoundaryPartsAssembleIntoAPlausibleArea` is the executable form of this
requirement: every oblast must be at least 1000 km², every city at least
5 km², every Sofia district at least 0.2 km², and each city polygon must
cover a known interior point (Plovdiv, Sofia, Varna, Burgas, Ruse). A
regeneration that reverts to one part per member way fails it immediately.

Vertex counts fall by roughly 20–25× — oblasti 369,993 → 14,051; cities
109,412 → 5,635; sofia-districts 26,457 → 1,394 — while staying far more precise
than point-in-polygon on a sensor coordinate requires.

Each feature's `properties` must carry `slug`, `name_bg`, `name_en` and
`source`. `area.Import` requires the first; the file must be a
`FeatureCollection` (a bare `Feature` imports zero features and once shipped
that way).

## Acceptance checks

A regeneration is finished when all of these pass — not when the files parse.

```bash
go test ./internal/area/ -run 'TestCommittedSlugs|TestSlugsAreUnique'   # no Docker needed
go test ./internal/area/                                                # needs Docker
```

- `TestCommittedSlugsFollowTheDocumentedRule` — every slug recomputes from
  `name_bg`, and the per-file feature counts are 28 / 27 / 24.
- `TestSlugsAreUniqueAcrossEveryFile` — 80 features, 80 distinct slugs.
- `TestAllFourFilesImportWithoutRowLoss` — per-**kind row counts in the
  database** after importing all four: country 1, oblast 28, city 27,
  neighbourhood 24. This is the one that catches a slug collision; per-file
  feature counts stay correct while rows are being overwritten.
- `TestSofiaSensorResolvesThroughAllTiers` — one Sofia point lands in an oblast,
  a city and a district. The three tiers must nest.
- `TestBoundariesDoNotSwapCoordinates` — bbox check. A file written `[lat, lon]`
  parses, imports and produces valid polygons; they just sit in the Indian
  Ocean.
- `TestImportCommittedBulgariaBoundary` — point-in-polygon against real cities,
  not a bbox: Bulgaria's bounding box overlaps five neighbours, so Bucharest,
  Thessaloniki and Skopje falling *outside* is what makes it a real test. Also
  covers Pomorie and Sinemorets, the two sensors the Natural Earth coastline
  used to cut off.
- `TestBoundaryPartsAssembleIntoAPlausibleArea` — every oblast/city/district
  area clears a minimum km², and each named city polygon covers its own known
  interior point. Catches a regeneration that keeps each relation member way as
  its own ring instead of assembling them with `ST_LineMerge`/`ST_BuildArea`.
- `TestCountryBoundaryCoversEveryOblast` — every oblast's
  `ST_PointOnSurface` must fall inside `bulgaria.geojson`. Catches a seam or
  gap left by unioning the 28 oblasti that a national-total-area check alone
  would miss, by naming the specific oblast that fails.
- `TestCountryBoundaryDoesNotOverlapNeighbours` — `bulgaria.geojson`'s
  `ST_Intersection` area with each of the five Natural Earth neighbour files
  must stay under 0.01 km². Catches a plain union (no `ST_Difference` pass)
  that would let `FilterByBoundary`'s `ORDER BY country_code LIMIT 1` tiebreak
  stamp neighbouring sensors as Bulgarian.

Then check the footer still attributes OpenStreetMap.

## Adding a fourth tier

Run the cross-file slug check before committing — the primary key does not care
which file a row came from. `TestSlugsAreUniqueAcrossEveryFile`'s `wantTotal`
and its file list both need updating, and a new tier needs its own suffix rule
if its names can collide with an existing tier's.
