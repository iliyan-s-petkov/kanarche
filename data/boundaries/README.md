# Boundary data

This file documents what these files **are**. Two companions:

- `docs/boundary-regeneration.md` — how to **rebuild** them: the Overpass
  queries, the PostGIS cleaning pipeline, the slug rule as an executable test,
  and the acceptance checks. Read it before regenerating anything.
- `docs/known-limitations.md` — the consequences a **reader of the map** should
  know, chiefly the 13/14 city-proper vs. municipality split below.

## The six `country`-kind files

`bulgaria.geojson`, plus `greece.geojson`, `north-macedonia.geojson`,
`romania.geojson`, `serbia.geojson` and `turkey.geojson` — one per code in
`airbg.yaml`'s `upstream.countries`. Importing them is a **hard prerequisite for
ingesting anything**: `collect` filters every incoming sensor against the
`country`-kind areas with `ST_Covers`, and until at least one enabled country
has a boundary it fails closed and stores zero rows. See
`docs/ingest-filter.md` for why the filter is geometric and what the allow list
does.

```bash
for c in bulgaria greece north-macedonia romania serbia turkey; do
  airbg import-areas data/boundaries/$c.geojson country
done
```

Run this once, after `airbg migrate`, before the first `collect`. A country
enabled in config with no boundary here ingests nothing and is named in a
startup warning.

Each file carries an `iso_a2` property, which is where `area.country_code` comes
from — the filter scopes its boundary set by that code, so a file without one is
rejected at import rather than imported invisibly. The `country` kind is
excluded from `area_sensor`, so these do not become browsable areas: a foreign
sensor appears in the hex grid and in no oblast or city aggregate.

**Source:** Five of the six — `greece.geojson`, `north-macedonia.geojson`,
`romania.geojson`, `serbia.geojson`, `turkey.geojson` — are
[Natural Earth](https://www.naturalearthdata.com/) 1:10m Admin 0 Countries,
geometry unmodified, one feature per `ADM0_A3` (`GRC`, `MKD`, `ROU`, `SRB`,
`TUR`). Natural Earth is public domain, so these are committed here rather
than left as a deployment step.

`bulgaria.geojson` is **not** from that release. It was regenerated
2026-09-27 as `ST_Union` of the 28 `oblasti.geojson` features (OSM, ODbL 1.0)
and the prior Natural Earth outline, then `ST_Difference`d against the union
of the five neighbour files above, all in a scratch PostGIS container (holes
left by per-oblast simplification seams smaller than 1 km² dropped). The
Natural Earth coastline cut into the Black Sea coast by 0.25–0.45 km,
dropping two live sensors — 7669 (Pomorie) and 32826 (Sinemorets) — that
`FilterByBoundary`'s `ST_Covers` silently rejected as outside Bulgaria. See
`docs/boundary-regeneration.md` for the rebuild steps and
`internal/area/area_test.go`'s `TestImportCommittedBulgariaBoundary` for the
regression coverage.

Mixing a Natural Earth boundary for the five neighbours with an OSM-derived
one for Bulgaria would otherwise mean the shared borders no longer come from
the same survey/vintage, and `FilterByBoundary`'s tiebreak (`ORDER BY
country_code LIMIT 1`) sorts `BG` before all five, so any sensor inside a
resulting overlap strip would be stamped `BG` rather than its true neighbour.
The `ST_Difference` step above removes that risk: `bulgaria.geojson`'s
intersection with each neighbour file is now under 0.01 km² (residual
floating-point noise from independently rounding two datasets to 6 decimal
places, not real overlap). `internal/area/committed_boundaries_test.go`'s
`TestCountryBoundaryDoesNotOverlapNeighbours` asserts this on every run.

| file | geometry | vertices | size |
|---|---|---|---|
| `bulgaria.geojson` | MultiPolygon, 10 parts | 1,290 | 28 KB |
| `greece.geojson` | MultiPolygon, 74 parts | 12,192 | 142 KB |
| `north-macedonia.geojson` | Polygon | 726 | 9 KB |
| `romania.geojson` | Polygon | 2,396 | 28 KB |
| `serbia.geojson` | Polygon | 1,880 | 22 KB |
| `turkey.geojson` | MultiPolygon, 6 parts | 7,280 | 85 KB |

Greece is a MultiPolygon because of the islands, and dropping the small parts to
save bytes would drop the sensors on them. Bulgaria's 10 parts are the
mainland plus nine small real exclaves already present in `burgas-oblast`'s
own OSM geometry (each well under 1 km², none a union artifact — see
`docs/boundary-regeneration.md`).

Verified by point-in-polygon before committing: Sofia, Varna, Plovdiv, Musala,
the far-eastern coastal towns Balchik and Shabla, and now Pomorie and
Sinemorets fall inside Bulgaria; London falls inside none of the six; and
Athens, Skopje, Bucharest, Belgrade and Istanbul each fall inside exactly
one, now that the `ST_Difference` step removes the shared-border overlap
strips a plain union would otherwise leave for `FilterByBoundary`'s `ORDER BY
country_code LIMIT 1` tiebreak to arbitrate. The neighbour checks matter —
Bulgaria's bounding box overlaps all five, so a box test would wrongly accept
every one of them.

### Not to be confused with the test fixture

`internal/area/testdata/bulgaria.geojson` is a deliberately crude hand-authored
22-vertex polygon used only by tests. It is materially wrong along the eastern
border (cut at longitude 28.00, so it drops Balchik, Kavarna and Shabla) and is
never loaded at runtime. Tests that assert "just outside the boundary" depend on
knowing its exact shape, so it stays as it is. Do not substitute this file for
it, and do not import that one in production.

## `oblasti.geojson`, `cities.geojson`, `sofia-districts.geojson`

The three area tiers `/api/v1/overview` serves. Import order does not matter,
but all three must be imported before the API reports meaningful coverage:

```bash
airbg import-areas data/boundaries/oblasti.geojson oblast
airbg import-areas data/boundaries/cities.geojson city
airbg import-areas data/boundaries/sofia-districts.geojson neighbourhood
```

**Source:** OpenStreetMap, via the Overpass API
(`overpass-api.de`/`overpass.kumi.systems`). Administrative levels, per Phase 1
§5.4's intent, with the actual levels OSM uses for each tier in Bulgaria (which
are not uniform — see below): 4 for oblasti, 8 or 5 for cities depending on
which level a given city is mapped at, 6 for Sofia's districts. Retrieved
2026-08-10.

**Oblasti (28 features):** `boundary=administrative`, `admin_level=4`, inside
the `ISO3166-1=BG` area. One relation per oblast; matches exactly.

**Cities (27 features, not 28):** Bulgaria has 28 oblasti but only 27 distinct
oblast-capital cities — Sofia is the administrative seat of both Sofia-grad
(admin_level 4, an oblast that *is* the city) and Sofia Oblast (admin_level 4,
a separate oblast whose `admin_centre` node sits at the same coordinates as
central Sofia, confirmed against this file's own Overpass response). A
28th "city" feature would mean committing Sofia's polygon twice under two
slugs — the same physical place claiming two rows — which is worse than a
file that honestly has 27. OSM's `admin_level=8` ("city proper") boundary,
tagged `place=city` or `place=town`, exists for only 13 of the 27 capitals
(Sofia, Ruse, Dobrich, Razgrad, Targovishte, Silistra, Shumen, Varna, Gabrovo,
Veliko Tarnovo, Kyustendil, Burgas, Vidin); the other 14 capitals
(Blagoevgrad, Vratsa, Kardzhali, Lovech, Montana, Pazardzhik, Pernik, Pleven,
Plovdiv, Sliven, Smolyan, Haskovo, Stara Zagora, Yambol) have no separate
settlement-level boundary in OSM at all, so their `admin_level=5`
(`border_type=municipality`) boundary is used instead — that covers the whole
municipality, not just the built-up city, which is an acceptable
approximation for point-in-polygon area aggregation but not a survey-grade
city outline.

**Sofia districts (24 features):** the 24 `raiони` are `boundary=administrative`
relations at `admin_level=6` inside the area named `Столична` (`admin_level=5`,
the "Stolichna" municipality — the brief's assumed name `Столична община` and
assumed `admin_level=9` do not match current OSM tagging; `admin_level=9`
under `Столична` is a finer subdivision, not the district level). Includes
`Лозенец` (Lozenets), which the importer test in
`internal/area/committed_boundaries_test.go` uses as its cross-tier point.

**Licence:** ODbL 1.0. Attribution — "© OpenStreetMap contributors" — must
appear in the site footer alongside sensor.community's. This is a licence
obligation, not a courtesy.

**Geometry:** fetched with `out geom;`, which returns one line geometry per
relation member way, not an assembled polygon. The member ways are stitched
into closed rings with `ST_LineMerge`/`ST_Collect` and turned into polygons
with `ST_BuildArea` *before* cleaning — skipping this step is what produced
the fragmented boundaries fixed by OpenProject #614 (21 of 27 cities were a
MultiPolygon of dozens of perimeter-tracing slivers instead of one assembled
polygon; see `docs/boundary-regeneration.md`). Then cleaned and simplified with
`ST_MakeValid`, `ST_SimplifyPreserveTopology(geom, 0.002)` (roughly 200 m), and
`ST_CollectionExtract(..., 3)` to drop stray linestrings. `ST_MakeValid` is
needed here — several relations contain a spurious near-zero-length "outer"
member way (an OSM digitisation artefact) that, treated as its own ring,
collapses into a degenerate line under simplification and turns the result
into a `GeometryCollection` that `area.Import`'s `validateGeometry` would
reject; `ST_MakeValid` before simplifying repairs that without hand-editing the
source rings. `ST_SimplifyPreserveTopology` rather than `ST_Simplify` because
plain simplification can also emit self-intersecting rings on its own.
Vertex counts dropped by roughly 20–25x (oblasti: 369,993 → 14,051; cities:
109,412 → 5,635; sofia-districts: 26,457 → 1,394) while staying far more
precise than point-in-polygon on a sensor coordinate requires.

**Slugs** are transliterated from `name_bg` to ASCII. They appear in URLs
(`/oblast/{slug}`), so they must be stable: changing a slug breaks every
inbound link and every search-engine result for that page. Treat them as
permanent once shipped.

### Slugs must be unique across every `kind`, not just within one file

`area.slug` is the table's `PRIMARY KEY`, shared globally across all `kind`
values, and `area.Import` upserts on it (`ON CONFLICT (slug) DO UPDATE SET
kind = EXCLUDED.kind, ...`). A Bulgarian oblast and its capital city share the
same name and therefore the same transliteration — "Варна" oblast and "Варна"
city both slugify to `varna` — so importing `oblasti.geojson` then
`cities.geojson`, exactly the order this file documents above, silently
rewrote 26 of 28 oblast rows into city rows: same slug, so the second import's
`kind`/`name_bg`/`name_en`/`geom` overwrote the first import's row rather than
inserting a new one. `SELECT count(*) FROM area WHERE kind = 'oblast'` dropped
from 28 to 2. This is caught by
`internal/area/committed_boundaries_test.go`'s
`TestAllFourFilesImportWithoutRowLoss`, which asserts per-kind row counts in
the database after all four files are imported together — not just each
file's parsed feature count, which stays correct regardless of what happens
in the table.

**Fix:** every oblast slug carries a uniform `-oblast` suffix
(`varna-oblast`, `plovdiv-oblast`, `sofia-grad-oblast`, `sofiyska-oblast`, all
28, including the two Sofia oblasti whose slugs never collided with a city in
the first place). The suffix is uniform rather than applied only where a
collision exists today, because a collision-driven rule breaks again the
first time a new city is added whose name happens to match an oblast that
currently has no colliding city. Cities keep their bare slug (`varna`,
`plovdiv`) because a city is what a reader searches for, and the slug is what
appears in its URL (`/area/varna`). Sofia district slugs are untouched.

Checked and confirmed collision-free (28+27+24+6 = 85 distinct slugs, the six
country slugs included):
oblast vs. city (the only pair that collided, now fixed by the suffix), city
vs. Sofia district, oblast vs. Sofia district, and every one of the three
files against `bulgaria.geojson`'s own `bulgaria` slug. Any future fourth or
fifth tier must run the same cross-file slug check before being committed —
the primary key does not care which file a row came from.
