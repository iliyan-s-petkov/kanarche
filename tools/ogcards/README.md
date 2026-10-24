Generates the per-area og:image cards (1200x630, 32-colour palette PNG) into `internal/web/static/og/<lang>/<slug>.png`. The PNGs are committed, CI does not regenerate. Needs `pngquant` and `cd web && npm ci`.

Run: `node tools/ogcards/gen.mjs`

Font: the page stack is `"IBM Plex Sans", "Helvetica Neue", Arial`. Plex is not installed on the generating machine, so the cards render in Helvetica Neue (the generator prints the resolved font).
