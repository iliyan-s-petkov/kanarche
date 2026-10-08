<div align="center">

<img src="internal/web/static/kanarche-mascot.svg" width="160" height="160" alt="Kanarche mascot: a samurai canary">

# Kanarche

**Real-time air quality in Bulgaria, from citizen sensors and official stations on one map.**

[Live map](https://kanarche.eu) · [Embed](docs/embedding.md) · [Docs](#documentation) · [Contributing](CONTRIBUTING.md)

</div>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/hero-dark.png">
  <img src="docs/images/hero-light.png" alt="The Kanarche map of Bulgaria in a desktop browser and on a phone: coloured hexagons showing PM2.5 medians by area, with wind streaks over the map.">
</picture>

## What and why

Hundreds of volunteers across Bulgaria run low-cost particulate sensors on
their balconies and roofs, and the state runs a few dozen reference stations.
Both publish open data, but neither is easy to read at a glance, and the two
are rarely seen side by side. Kanarche puts them on one map, in Bulgarian and
English, so anyone can see what the air is like where they live right now and
how it has changed.

**Goals**

- Show the air in a place in one look, without an account, an app or an ad.
- Be honest about the data: medians instead of single outliers, coverage shown
  next to every figure, and a plain [account of what the map cannot
  tell you](docs/known-limitations.md).
- Stay open: open source, open data in, attributed data out, and a map any
  school, newsroom or municipality can embed.

**Vision.** A shared public picture of Bulgaria's air that is good enough for a
parent deciding whether to open a window and for a community asking its
municipality to act — built from sensors that neighbours host, and improving as
more of them join.

## Features

- **Two networks, one map.** Citizen sensors from sensor.community and the
  official ИАОС stations published through the European Environment Agency,
  each switchable in the Layers menu.
- **Medians by area.** Hexagon cells show the median of the ground beneath
  them at country and city zoom, and individual sensors inside an area.
- **Every province, city and Sofia district** has its own page with history
  charts up to a year and a text table of the same readings.
- **More than PM.** PM2.5, PM10, temperature, humidity, pressure and noise,
  plus six gases from the official stations; averaging windows from now to a
  week, and hourly playback.
- **Wind forecast overlay**, marked as a forecast rather than a measurement.
- **Embeddable** with one `<iframe>`, in light and dark themes, on a
  self-hosted basemap.

## Data and attribution

- Citizen sensor data: [sensor.community](https://sensor.community/) contributors, ODbL 1.0
- Official station data: Executive Environment Agency (ИАОС) via the
  [European Environment Agency](https://www.eea.europa.eu/)'s air quality programme
- Bathing-water quality: [European Environment Agency](https://www.eea.europa.eu/)
  Bathing Water Directive data (Discodata, plus the 2025 Datahub workbook), CC BY 4.0
- Boundaries and basemap: © [OpenStreetMap](https://www.openstreetmap.org/copyright) contributors, ODbL 1.0
- National outline: [Natural Earth](https://www.naturalearthdata.com/), public domain
- Wind forecast: [Open-Meteo](https://open-meteo.com/), ECMWF IFS model, CC BY 4.0
- Hosting kindly donated by [Hostellation](https://hostellation.com/)

Before comparing two places, read
[docs/known-limitations.md](docs/known-limitations.md): some cities are drawn
as the city proper and others as the whole municipality.

## Contributing

This is a volunteer project and there is room for more than code. Report a
sensor or a number that looks wrong, host a sensor through
[sensor.community](https://sensor.community/), translate the site (all copy
lives in one JSON file per language in `internal/i18n/`), or pick up an issue.
[CONTRIBUTING.md](CONTRIBUTING.md) explains how.

## Documentation

| Document | Covers |
|---|---|
| [development.md](docs/development.md) | Running locally, the four test tiers |
| [operations.md](docs/operations.md) | Subcommands, how `serve` works, endpoints, database, container image |
| [configuration.md](docs/configuration.md) | Every `airbg.yaml` key and `AIRBG_*` override |
| [deployment.md](docs/deployment.md) | The production host, step by step |
| [embedding.md](docs/embedding.md) | Putting the map on another site |
| [known-limitations.md](docs/known-limitations.md) | What the data cannot tell you |
| [ingest-filter.md](docs/ingest-filter.md) | Which readings are stored, and why |
| [map-rendering.md](docs/map-rendering.md) | Why the map looks the way it does |
| [wind-overlay.md](docs/wind-overlay.md) | The wind layer and its caveats |
| [tiles.md](docs/tiles.md) | Generating the self-hosted basemap |
| [boundary-regeneration.md](docs/boundary-regeneration.md) | Rebuilding the area boundaries |
| [design/DESIGN.md](docs/design/DESIGN.md) | The visual design contract |

## Run it locally

Needs Go, Node.js and Docker.

```bash
docker compose up -d db
export AIRBG_DATABASE_URL='postgres://airbg:airbg@localhost:5432/airbg?sslmode=disable'
export AIRBG_CONFIG="$PWD/airbg.yaml"
go run ./cmd/airbg migrate
go run ./cmd/airbg import-areas data/boundaries/bulgaria.geojson country
(cd web && npm ci --ignore-scripts && npm run build)
go run ./cmd/airbg serve        # http://localhost:8080
```

`docker-compose.yml` is for development only. The `import-areas` step is
required, or nothing is stored. Details, tests and the degraded no-frontend
mode are in [docs/development.md](docs/development.md).

## License

[MIT](LICENSE). The data keeps its own licences, listed above.
