// Floating back-to-top button, shown once the map (or, on a map-less page, the first screen) has scrolled out through the top.
export function createBackToTop({ doc = document, win = window, IO = win.IntersectionObserver } = {}) {
  const btn = doc.querySelector('.back-to-top')
  const target = doc.querySelector('.map-shell') ?? doc.querySelector('.back-to-top-sentinel')
  if (!btn || !target || !IO) return null

  // The observed band is the lower half of the viewport; a target above it (bottom <= band top) shows the button.
  // A hero map that never leaves the viewport still qualifies; a target below the fold does not.
  const observer = new IO(([entry]) => {
    btn.classList.toggle('back-to-top--visible', !entry.isIntersecting && entry.boundingClientRect.bottom <= entry.rootBounds.top)
  }, { threshold: 0, rootMargin: '-50% 0px 0px 0px' })
  observer.observe(target)

  btn.addEventListener('click', () => {
    const reduce = win.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
    win.scrollTo({ top: 0, behavior: reduce ? 'instant' : 'smooth' })
    // The page heading is the first thing a screen reader should land on; preventScroll keeps the smooth scroll going.
    const heading = doc.querySelector('main h1')
    if (heading) {
      if (!heading.hasAttribute('tabindex')) heading.setAttribute('tabindex', '-1')
      heading.focus({ preventScroll: true })
    }
  })

  return { observer }
}
