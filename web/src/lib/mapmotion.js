// Camera moves animate unless the visitor asked for reduced motion (WCAG 2.3.3).
// Every flyTo/easeTo/fitBounds/zoomIn/zoomOut option bag goes through here.
const reduced = () => globalThis.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false

export function motion(options) {
  return reduced() ? { ...options, animate: false } : options
}
