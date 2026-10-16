// A printed unit read aloud: "µg/m³" is heard as "micro g m cubed", so labels the
// browser builds for a screen reader swap in the words the server publishes in the
// head (see internal/web/spoken_units.go). The symbol stays on screen.
const SELECTOR = 'meta[name="spoken-units"]'

export function readSpokenUnits(doc = globalThis.document) {
  const raw = doc?.querySelector(SELECTOR)?.getAttribute('content')
  if (!raw) return {}
  try {
    const table = JSON.parse(raw)
    return table && typeof table === 'object' ? table : {}
  } catch {
    return {}
  }
}

// spokenUnit(unit) -> the words for a symbol, or the symbol itself when it has none.
export function spokenUnit(unit, doc = globalThis.document) {
  if (!unit) return ''
  return readSpokenUnits(doc)[unit] ?? unit
}
