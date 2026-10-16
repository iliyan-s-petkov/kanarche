import { HEX_OUTLINE_LAYER_ID, HEX_POINT_LAYER_ID, LAYER_ID, OFFICIAL_LAYER_ID } from './mapids.js'

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

// Strokes a band swatch's shape at its class weight. The stroke is doubled and unscaled
// because the svg clips its outer half, which leaves an inset of exactly the weight.
// The class-outline rule in app.css repaints the stroke in the theme's ink; the attribute is the fallback.
export function outlineSwatch(shape, index, ink) {
  shape.classList.add('class-outline')
  shape.setAttribute('stroke', ink || 'currentColor')
  shape.setAttribute('stroke-width', String(2 * classOutlineWidth(index)))
  shape.setAttribute('vector-effect', 'non-scaling-stroke')
}

// The outline ink for the current theme: --class-outline-ink (the theme's --fg), else `fallback`.
export function outlineInk(doc, fallback) {
  const root = doc?.documentElement
  const ink = root ? doc.defaultView.getComputedStyle(root).getPropertyValue('--class-outline-ink').trim() : ''
  return ink || fallback
}

// Map paint that draws a class outline or halo in the ink.
const INKED = [
  [HEX_OUTLINE_LAYER_ID, 'line-color'],
  [HEX_POINT_LAYER_ID, 'circle-stroke-color'],
  [LAYER_ID, 'circle-stroke-color'],
  [OFFICIAL_LAYER_ID, 'icon-halo-color'],
]

export function applyOutlineInk(map, ink) {
  for (const [id, prop] of INKED) {
    if (map.getLayer(id)) map.setPaintProperty(id, prop, ink)
  }
}

// Calls `onChange` when the theme switches: the picker's data-theme, or the OS scheme under "auto". Returns the stop.
export function watchTheme(doc, win, onChange) {
  const observer = new win.MutationObserver(() => onChange())
  observer.observe(doc.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
  const media = win.matchMedia?.('(prefers-color-scheme: dark)')
  media?.addEventListener?.('change', onChange)
  return () => {
    observer.disconnect()
    media?.removeEventListener?.('change', onChange)
  }
}
