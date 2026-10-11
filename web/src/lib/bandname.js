// The words for a reading's band, so a colour is never the only thing that says it.

// The scale's English label -> the dataset key of its data-t-band-* attribute.
const BAND_ATTRS = {
  Good: 'tBandGood',
  Fair: 'tBandFair',
  Moderate: 'tBandModerate',
  Poor: 'tBandPoor',
  'Very poor': 'tBandVeryPoor',
  'Extremely poor': 'tBandExtremelyPoor',
}

// readBandNames(dataset) -> { [scale label]: page-language name }.
export function readBandNames(d) {
  return Object.fromEntries(Object.entries(BAND_ATTRS).map(([label, attr]) => [label, d[attr] || '']))
}

// bandName(value, bands, names) -> the name of the band a value falls in, or ''.
// Upper is inclusive and a null upper is the open top, as in internal/web/colour.go.
export function bandName(value, bands, names) {
  if (typeof value !== 'number' || Number.isNaN(value)) return ''
  const band = bands.find((b) => b.upper === null || b.upper === undefined || value <= b.upper)
  return band ? names[band.label] || '' : ''
}
