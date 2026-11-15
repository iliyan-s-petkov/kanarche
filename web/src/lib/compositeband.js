// The EU AQI level of a site: the worse of its PM2.5 and PM10 bands, as the EAQI rates it.

export const METRICS = ['P2', 'P1']

// Index of the band a value falls in, -1 for no value. Same rule as bandName.
function bandIndex(value, bands) {
  if (typeof value !== 'number' || !Array.isArray(bands)) return -1
  return bands.findIndex((b) => b.upper === null || b.upper === undefined || value <= b.upper)
}

// compositeBand(values {P2, P1}, bands {P2, P1}, names) -> the page-language name of the
// worse band, or '' when neither metric has a reading.
export function compositeBand(values, bands, names) {
  let worst = { i: -1, label: '' }
  for (const m of METRICS) {
    const i = bandIndex(values?.[m], bands?.[m])
    if (i > worst.i) worst = { i, label: bands[m][i].label }
  }
  return worst.i < 0 ? '' : names?.[worst.label] || ''
}
