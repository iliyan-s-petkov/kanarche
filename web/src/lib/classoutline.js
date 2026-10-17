// Outline weight per air-quality class: the second channel beside colour (A11Y-05).
// Index is the band's position in the served table, best first; CSS px. Mirrored as --class-outline-N in both theme.css.
export const CLASS_OUTLINE_PX = [0.75, 1.5, 2.25, 3, 3.75, 4.5]

const TOP = CLASS_OUTLINE_PX.length - 1

// bandIndex(value, bands) -> the band a value falls in, or -1 with no reading.
// Upper is inclusive and a null upper is the open top, as in bandName and internal/web/colour.go.
export function bandIndex(value, bands) {
  if (typeof value !== 'number' || Number.isNaN(value) || !bands?.length) return -1
  return bands.findIndex((b) => b.upper === null || b.upper === undefined || value <= b.upper)
}

// No class draws the lightest weight; a table longer than the ladder tops out at the heaviest.
export function classOutlineWidth(index) {
  return CLASS_OUTLINE_PX[Math.max(0, Math.min(TOP, index))]
}

// The same lookup as a MapLibre expression over the feature's `band`, plus `extra` px.
export function classOutlineExpr(extra = 0) {
  const index = ['max', 0, ['min', TOP, ['to-number', ['coalesce', ['get', 'band'], 0]]]]
  const width = ['at', index, ['literal', CLASS_OUTLINE_PX]]
  return extra ? ['+', width, extra] : width
}

// Marks a band swatch's shape for its class outline. Colour and weight come from the
// .class-outline[data-class] rules in app.css, doubled because the svg clips its outer half.
export function outlineSwatch(shape, index) {
  shape.classList.add('class-outline')
  shape.setAttribute('data-class', String(Math.max(0, Math.min(TOP, index))))
  shape.setAttribute('vector-effect', 'non-scaling-stroke')
}
