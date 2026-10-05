# The wind overlay

An optional map layer showing forecast wind on the model's own 0.25 degree
grid over Bulgaria. It is the only layer in this application that is **not a
measurement**, and most of the decisions below exist to keep that distinction
visible rather than convenient.

## Why a forecast at all

sensor.community has no anemometers. Every sensor we ingest reports some subset
of PM, temperature, humidity, pressure and noise; none of them reports wind. So
the choice was between a met model, a derivation, or nothing.

Deriving geostrophic wind from our own pressure field was considered and
rejected. In this terrain a pressure gradient across two sensors is mostly an
*altitude* gradient — Sofia sits in a basin at 550 m ringed by mountains over
2000 m — so the derived vector would be an artefact of topography, and it would
arrive on the map wearing the same clothes as our measured data. A wrong number
that looks locally sourced is worse than a right number that is openly external.

## Provider

Open-Meteo, model `ecmwf_ifs025`, over plain JSON:

```
GET https://api.open-meteo.com/v1/forecast
    ?latitude=<a,b,c>&longitude=<x,y,z>
    &hourly=wind_speed_10m,wind_direction_10m
    &models=ecmwf_ifs025&wind_speed_unit=ms&timezone=UTC
```

No API key, no new Go module — a `net/http` GET and `encoding/json`, the same
shape as `internal/upstream`. ECMWF's own Open Data service was the alternative
and serves GRIB2, which cannot be decoded without either a dependency or a lot
of hand-rolled binary parsing; the standing rule is no new third-party
dependency.

### Two properties of the response that the code depends on

**Results come back in request order, and their coordinates are not yours.**
The response is a JSON array, one object per requested point, in the order
requested. Each object's `latitude`/`longitude` are the *model grid cell* the
point fell into, not what was asked for: request 42.7/23.3 and the answer says
42.75/23.25. Matching responses to points by coordinate therefore does not work
— the answer names the snapped cell, which need not equal the request.
The client keys results by **request index** and asserts the array length
matches the request. `TestResponseIsKeyedByIndexNotCoordinate` pins this.

**The lattice is the model's grid.** `ecmwf_ifs025` is a 0.25° grid, about
20 km east-west and 28 km north-south at this latitude. The queried points are
multiples of 0.25°, so each one is a distinct native cell and nothing is
upsampled on the server. The on-map label names the model and its resolution.
The payload's `resolution_km` is the nominal lattice spacing (25).

## Storage

Wind is stored, not proxied. A cache-through proxy would have meant an upstream
outage rendering a blank layer and no history at all; storing it makes the
overlay behave like every other datum in the application — the API reads our
own database, and a fetch failure degrades to stale data with an honest
timestamp rather than to nothing.

Migration `00010` adds a `wind_forecast` hypertable keyed by `(hex_q, hex_r,
valid_at)`. The columns keep their original names but now hold the **lattice
index**: `hex_q` is longitude and `hex_r` is latitude in units of 0.25°
(`21.75E 42.75N` is `87, 171`). An integer index is the grid's identity where a
float centre is not. The table records the resolution rows were written at, and
reads filter on it: rows from the former 15 km hex grid (`resolution_km = 15`)
are ignored and age out with the retention policy.

Retention is short. A forecast that has been superseded is not history worth
keeping — we keep enough to serve the current overlay and to see what the model
said versus what the PM did, and no more.

## Which points get queried

A fixed lattice over Bulgaria's bounding box plus a 0.5° margin, independent of
which sensors exist (`snapshot.WindLattice`): longitude 21.75-29.25, latitude
40.75-44.75, 31 x 17 = 527 points. The margin is there so the borders and the
Black Sea coast have wind. A sensor-driven set left the Danube plain and the
north-east thin or empty.

Points are batched per request (`points_per_request`, 100), so a collection is
6 requests, every three hours. ECMWF IFS updates four times a day, and each
run writes 24 hourly rows, so the current hour is served from the last run until
the next one lands.

## Rendering

By default the layer renders as animated streaks on a canvas over the map
(`web/src/lib/windstreaks.js`). Under `prefers-reduced-motion: reduce`, or with
`?windfx=arrows`, it falls back to the static arrows below, so the arrows get
built either way.

Arrows: one per lattice point, rotated to the wind direction, scaled by speed.

Direction follows the meteorological convention Open-Meteo uses: the direction
the wind is coming **from**. The arrow is drawn pointing the way the air is
going, which is the opposite, and this is exactly the sort of thing that is
wrong in production for a year, so it is pinned by a test rather than by a
comment.

The arrow is the character U+2192 in a MapLibre symbol layer, not a sprite
image: the marker labels already render text through the style's own glyph
source, so a glyph needs neither `addImage` nor an SDF sprite to tint, and it
is one fewer build artefact to ship and keep in sync. The glyph points east, so
the layer rotates it by `bearing - 90`; the feature property stays a compass
bearing, because that is what every other surface in this codebase calls it.

The layer is off by default and toggled by a button in the map's bottom-right
corner. It is fetched once, on first switch-on, and cached for the page's
lifetime — the payload is a single forecast hour for the whole country, so it
does not change while the visitor pans, and it is deliberately not tied to the
viewport, the tier, or the selected metric. A 503 (no forecast covers the
current hour) leaves the layer off and raises nothing: that is an ordinary
state for an optional overlay, and the map's error banner belongs to the data
the page exists to show.

## Labelling

Whenever the layer is on, a persistent, translated line names the model, its
grid resolution, and the forecast's valid time. Not behind an info icon, not
only in the legend: the point of choosing a met API over a derivation was to
avoid presenting inference as measurement, and a disclosure the user has to go
looking for does not achieve that.

## Configuration

Under `wind:` in `airbg.yaml`. The layer is off unless configured — an operator
running this application without a met provider gets a site with no wind
overlay, not a site with a broken one.
