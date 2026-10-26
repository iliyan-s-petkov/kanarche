// The four readout cards an open sensor puts in place of the national ones.
//
// Same shape as the server's Readout (internal/web/render.go) — label, value,
// unit, tier and an optional band — so one component renders either set and
// the swap cannot change how a card looks, only what it says.

// The catalogue key under t.band for each api.Scales band label, mirroring
// bandLabelKeys in internal/web/colour.go.
const BAND_KEY = {
  Good: 'good', Fair: 'fair', Moderate: 'moderate', Poor: 'poor',
  'Very poor': 'veryPoor', 'Extremely poor': 'extremelyPoor',
}

// The health class a reading falls in, for the bar and word under a card.
// Only the EAQI tables are health classes: benzene and CO carry an axis, so
// they get no bar, mirroring healthBand in internal/web/colour.go.
export function bandFor(scale, value, t) {
  if (!scale || scale.name !== 'eaqi') return null
  for (const band of scale.bands ?? []) {
    if (band.upper == null || value <= band.upper) {
      const word = t?.band?.[BAND_KEY[band.label]]
      return word ? { colour: band.colour, word } : null
    }
  }
  return null
}

// Upper is inclusive, so a value exactly on a boundary belongs to the lower
// band; a null upper is the open-ended top.
export function bandColour(bands, value) {
  for (const band of bands ?? []) {
    if (band.upper == null || value <= band.upper) return band.colour
  }
  return ''
}

function fill(text, values) {
  return Object.entries(values).reduce(
    (s, [key, v]) => s.replaceAll(`{${key}}`, String(v)),
    text ?? '',
  )
}

// Page number format: locale separators, at most one decimal.
export function number(value, lang) {
  return value.toLocaleString(lang === 'bg' ? 'bg-BG' : 'en-GB', { maximumFractionDigits: 1 })
}

// areaCards returns [] when there is nothing honest to say — no stats, or a
// metric the open sensor's area does not report — and the caller then leaves
// the national strip standing rather than showing four blanks.
export function areaCards(stats, { metric, metricLabel, scale, area, lang, t }) {
  if (!stats) return []
  const unit = scale?.unit ?? ''
  const where = area ? fill(t.areaSensors, { area, total: stats.total }) : fill(t.sensorsOnly, { total: stats.total })

  const figure = (labelKey, value) => ({
    label: fill(t[labelKey], { metric: metricLabel }),
    metric: metricLabel,
    value: number(value, lang),
    unit,
    tier: where,
    band: bandFor(scale, value, t),
  })

  const cards = [
    figure('high', stats.high),
    figure('low', stats.low),
    figure('median', stats.median),
  ]

  // The fourth card is this sensor's place among the other three. Dropped when
  // the sensor reports nothing for the metric: a rank of nothing is not a rank,
  // and the area's own three figures are still true without it.
  if (stats.rank != null) {
    const above = stats.value - stats.median
    cards.push({
      label: t.thisSensor,
      value: String(stats.rank),
      unit: fill(t.ofTotal, { total: stats.total }),
      tier: above > 0 ? t.aboveMedian : above < 0 ? t.belowMedian : t.atMedian,
      band: null,
    })
  }
  return cards
}

// The t key for a network name, matching the server's
// strings.ReplaceAll(src, ".", "_") in internal/web/render.go.
const SOURCE_LABEL = {
  eea: 'sourceOfficial',
  'sensor.community': 'sourceCommunity',
}

// areaSourceCards mirrors AreaReadouts' breakdown group: one card per row.
// areaSourceStats owns the "is there a breakdown at all" decision, so a lone
// row here is a network whose neighbour is silent on this metric.
export function areaSourceCards(rows, { metricLabel, scale, lang, t }) {
  if (!Array.isArray(rows) || rows.length === 0) return []
  const unit = scale?.unit ?? ''

  return rows.map((row) => {
    const group = t[SOURCE_LABEL[row.source]] ?? row.source
    const tier = row.n === 1 ? t.sourceRowOne : fill(t.sourceRow, { n: row.n })
    return {
      label: metricLabel,
      metric: metricLabel,
      value: number(row.median, lang),
      unit,
      tier,
      band: bandFor(scale, row.median, t),
      group,
    }
  })
}
