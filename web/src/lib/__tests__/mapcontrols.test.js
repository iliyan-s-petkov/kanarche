// @vitest-environment jsdom
//
// jsdom, because every assertion here is on a real element: the class the kit
// styles, the aria-label an icon-only control needs, the disabled property.
// What jsdom does NOT have is the Fullscreen API, which is why mountFullscreen
// takes its document as an argument.
import { describe, it, expect, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { mountFullscreen, mountZoom, mountLocate, mountReset, installZoom, mountOrientation, installOrientation } from '../mapcontrols.js'

const frame = () => {
  const el = document.createElement('div')
  el.className = 'map map--hero'
  document.body.append(el)
  return el
}

// A document stand-in, because jsdom implements neither requestFullscreen nor
// fullscreenElement — so without one there is no way to reach either branch.
const fakeDoc = () => ({
  fullscreenElement: null,
  exitFullscreen: vi.fn(function () { this.fullscreenElement = null }),
  body: document.createElement('body'),
  addEventListener: vi.fn(),
})

describe('mountFullscreen', () => {
  it('carries the kit classes and both glyphs, and starts unpressed', () => {
    const el = frame()
    const btn = mountFullscreen(el, { label: 'Full screen', exitLabel: 'Exit' }, fakeDoc())

    expect(btn.parentElement).toBe(el)
    expect(btn.className).toBe('map__full')
    expect(btn.type).toBe('button')
    expect(btn.getAttribute('aria-pressed')).toBe('false')
    // Both icons ship at once; the kit's CSS shows one per pressed state, so a
    // JS swap here would be a second record of the same fact.
    expect([...btn.querySelectorAll('svg')].map((s) => s.getAttribute('class'))).toEqual([
      'map__full-ico map__full-ico--in',
      'map__full-ico map__full-ico--out',
    ])
  })

  // The key, the layers menu and everything else useful live outside the frame;
  // in real fullscreen the frame IS the viewport and nothing outside it renders.
  // Somebody has to be told when that happens, and the button is the only thing
  // that already knows — it recomputes on every enter, exit and
  // fullscreenchange.
  it('reports the state to a caller on every change, not just to itself', async () => {
    const el = frame()
    const doc = fakeDoc()
    const seen = []
    mountFullscreen(el, { label: 'F', exitLabel: 'E', onChange: (on) => seen.push(on) }, doc)
    expect(seen, 'never reported the starting state').toEqual([false])

    el.requestFullscreen = () => { doc.fullscreenElement = el; return Promise.resolve() }
    el.querySelector('.map__full').click()
    await Promise.resolve()
    expect(seen.at(-1)).toBe(true)

    el.querySelector('.map__full').click()
    expect(seen.at(-1)).toBe(false)
  })

  it('reports the faux-fullscreen fallback too', async () => {
    const el = frame()
    const doc = fakeDoc()
    const seen = []
    mountFullscreen(el, { label: 'F', exitLabel: 'E', onChange: (on) => seen.push(on) }, doc)

    el.requestFullscreen = () => Promise.reject(new Error('denied'))
    el.querySelector('.map__full').click()
    await Promise.resolve().then(() => {}).then(() => {})
    expect(seen.at(-1)).toBe(true)
  })

  it('names the button after what the next click does, not after the state', () => {
    const el = frame()
    const doc = fakeDoc()
    const btn = mountFullscreen(el, { label: 'Full screen', exitLabel: 'Exit' }, doc)
    expect(btn.getAttribute('aria-label')).toBe('Full screen')
    expect(btn.getAttribute('title')).toBe('Full screen')

    el.requestFullscreen = () => { doc.fullscreenElement = el; return Promise.resolve() }
    btn.click()
    return Promise.resolve().then(() => {
      expect(btn.getAttribute('aria-pressed')).toBe('true')
      expect(btn.getAttribute('aria-label')).toBe('Exit')
    })
  })

  it('falls back to the pinned frame when the browser refuses fullscreen', async () => {
    const el = frame()
    const doc = fakeDoc()
    const btn = mountFullscreen(el, { label: 'Full screen', exitLabel: 'Exit' }, doc)

    el.requestFullscreen = () => Promise.reject(new Error('refused'))
    btn.click()
    await Promise.resolve()
    await Promise.resolve()

    // The whole point of the fallback: a refusal must still change something.
    expect(el.classList.contains('map--faux-full')).toBe(true)
    expect(doc.body.classList.contains('has-faux-full')).toBe(true)
    expect(btn.getAttribute('aria-pressed')).toBe('true')
  })

  it('falls back where requestFullscreen does not exist at all', () => {
    const el = frame()
    const doc = fakeDoc()
    const btn = mountFullscreen(el, { label: 'Full screen', exitLabel: 'Exit' }, doc)
    btn.click()
    expect(el.classList.contains('map--faux-full')).toBe(true)
  })

  it('leaves the fallback on a second click', () => {
    const el = frame()
    const doc = fakeDoc()
    const btn = mountFullscreen(el, { label: 'Full screen', exitLabel: 'Exit' }, doc)
    btn.click()
    btn.click()
    expect(el.classList.contains('map--faux-full')).toBe(false)
    expect(doc.body.classList.contains('has-faux-full')).toBe(false)
    expect(btn.getAttribute('aria-pressed')).toBe('false')
  })

  it('exits real fullscreen through the document, not by dropping a class', () => {
    const el = frame()
    const doc = fakeDoc()
    doc.fullscreenElement = el
    const btn = mountFullscreen(el, { label: 'Full screen', exitLabel: 'Exit' }, doc)
    expect(btn.getAttribute('aria-pressed')).toBe('true')
    btn.click()
    expect(doc.exitFullscreen).toHaveBeenCalled()
  })
})

describe('mountZoom', () => {
  it('builds the kit stack: two named buttons, in and out', () => {
    const el = frame()
    const { el: bar, buttons } = mountZoom(el, { inLabel: 'In', outLabel: 'Out' })

    expect(bar.parentElement).toBe(el)
    expect(bar.className).toBe('map-zoom')
    expect([...bar.children].map((b) => b.getAttribute('data-act'))).toEqual(['in', 'out'])
    expect([...bar.children].map((b) => b.getAttribute('aria-label'))).toEqual(['In', 'Out'])
    // Icon-only, so title carries the same name for a pointer reader.
    expect([...bar.children].map((b) => b.getAttribute('title'))).toEqual(['In', 'Out'])
    for (const b of Object.values(buttons)) {
      expect(b.className).toBe('map-zoom__btn')
      expect(b.type).toBe('button')
      expect(b.querySelector('svg').getAttribute('aria-hidden')).toBe('true')
    }
  })

  it('gives each button its own glyph', () => {
    const el = frame()
    const { buttons } = mountZoom(el, { inLabel: 'In', outLabel: 'Out' })
    const paths = (b) => [...b.querySelectorAll('path')].map((p) => p.getAttribute('d'))
    // Plus is minus with a stroke added.
    expect(paths(buttons.in)).toHaveLength(2)
    expect(paths(buttons.out)).toHaveLength(1)
  })
})

describe('mountLocate', () => {
  it('is an icon button with a name, not a word over the attribution', () => {
    const el = frame()
    const button = mountLocate(el, { label: 'Намери ме' })

    expect(button.parentElement).toBe(el)
    expect(button.className).toBe('map-locate')
    expect(button.type).toBe('button')
    // No text: a label beside the attribution is what made the two read as one
    // control. The name moves to the places an icon-only control keeps it.
    expect(button.textContent).toBe('')
    expect(button.getAttribute('aria-label')).toBe('Намери ме')
    expect(button.getAttribute('title')).toBe('Намери ме')
    expect(button.querySelector('svg').getAttribute('aria-hidden')).toBe('true')
  })

  it('draws a crosshair, not one of the zoom stack marks', () => {
    const el = frame()
    const zoom = mountZoom(el, { inLabel: 'In', outLabel: 'Out' })
    const button = mountLocate(el, { label: 'Locate' })
    const paths = (b) => [...b.querySelectorAll('path')].map((p) => p.getAttribute('d'))

    expect(paths(button).length).toBeGreaterThan(0)
    for (const b of Object.values(zoom.buttons)) {
      expect(paths(button)).not.toEqual(paths(b))
    }
  })
})

describe('installZoom', () => {
  const fakeMap = (zoom = 7, min = 5, max = 18) => {
    const listeners = {}
    return {
      zoom,
      zoomIn: vi.fn(function () { this.zoom++; listeners.zoom?.() }),
      zoomOut: vi.fn(function () { this.zoom--; listeners.zoom?.() }),
      flyTo: vi.fn(),
      getZoom() { return this.zoom },
      getMinZoom: () => min,
      getMaxZoom: () => max,
      on: (name, fn) => { listeners[name] = fn },
      fire: (name) => listeners[name]?.(),
    }
  }

  const wire = (map) => {
    const { buttons } = mountZoom(frame(), { inLabel: 'In', outLabel: 'Out' })
    const reset = mountReset(frame(), { label: 'Reset' })
    installZoom(map, { ...buttons, reset }, { centre: [25.4858, 42.7339], zoom: 7 })
    return { ...buttons, reset }
  }

  it('drives the camera rather than holding its own zoom', () => {
    const map = fakeMap()
    const buttons = wire(map)
    buttons.in.click()
    buttons.out.click()
    expect(map.zoomIn).toHaveBeenCalledWith({}, { userInitiated: true })
    expect(map.zoomIn).toHaveBeenCalledOnce()
    expect(map.zoomOut).toHaveBeenCalledOnce()
  })

  it('returns the camera to the view the page opened at', () => {
    const map = fakeMap()
    wire(map).reset.click()
    // `center`, MapLibre's spelling, out of `centre`, ours. The two sit one
    // property apart and a map handed the wrong key flies to null island.
    expect(map.flyTo).toHaveBeenCalledWith(
      { center: [25.4858, 42.7339], zoom: 7, pitch: 0, bearing: 0 },
      { userInitiated: true },
    )
  })

  it('zoom and reset skip animation under reduced motion', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    try {
      const map = fakeMap()
      const buttons = wire(map)
      buttons.in.click()
      buttons.out.click()
      buttons.reset.click()
      expect(map.zoomIn).toHaveBeenCalledWith({ animate: false }, { userInitiated: true })
      expect(map.zoomOut).toHaveBeenCalledWith({ animate: false }, { userInitiated: true })
      expect(map.flyTo.mock.calls[0][0].animate).toBe(false)
    } finally {
      vi.unstubAllGlobals()
    }
  })

  it('reset flattens a tilted, rotated camera in one flight', () => {
    const map = fakeMap()
    map.pitch = 45
    map.bearing = 30
    // Apply the target the way MapLibre would, so the assertion is on the end state.
    map.flyTo = vi.fn((o) => { map.pitch = o.pitch; map.bearing = o.bearing; map.zoom = o.zoom })
    wire(map).reset.click()
    expect(map.flyTo).toHaveBeenCalledOnce()
    expect([map.pitch, map.bearing, map.zoom]).toEqual([0, 0, 7])
  })

  it('disables a button at the limit it reports, not at a limit written here', () => {
    const buttons = wire(fakeMap(5, 5, 18))
    expect(buttons.out.disabled).toBe(true)
    expect(buttons.in.disabled).toBe(false)

    const top = wire(fakeMap(18, 5, 18))
    expect(top.in.disabled).toBe(true)
    expect(top.out.disabled).toBe(false)
  })

  it('repaints the limits as the camera moves', () => {
    const map = fakeMap(6, 5, 18)
    const buttons = wire(map)
    expect(buttons.out.disabled).toBe(false)
    buttons.out.click()
    expect(buttons.out.disabled).toBe(true)
  })

  it('never disables reset: a panned camera still has somewhere to go back to', () => {
    expect(wire(fakeMap(7)).reset.disabled).toBe(false)
    expect(wire(fakeMap(5, 5, 18)).reset.disabled).toBe(false)
    expect(wire(fakeMap(18, 5, 18)).reset.disabled).toBe(false)
  })
})

// Same guard the key carries: every kit class this module writes has to exist
// in the kit's own stylesheet. A misspelling here is silent — the control still
// mounts, unstyled, in the top-left corner of the map.
it('writes only classes the kit defines', () => {
  // join(dirname(fileURLToPath(...))), not new URL(..., import.meta.url) — Vite
  // turns the latter into an asset import and then rejects the path for being
  // outside the project root. Same idiom as legend.test.js's own kit guard.
  const here = dirname(fileURLToPath(import.meta.url))
  const css = readFileSync(join(here, '..', '..', '..', '..', 'design-kit', 'components.css'), 'utf8')
  const el = frame()
  mountFullscreen(el, { label: 'a', exitLabel: 'b' }, fakeDoc())
  mountZoom(el, { inLabel: 'a', outLabel: 'b' })

  const used = new Set()
  for (const node of el.querySelectorAll('*')) {
    const cls = node.getAttribute('class') ?? ''
    for (const c of cls.split(/\s+/).filter(Boolean)) if (c.includes('__') || c.includes('--')) used.add(c)
  }
  expect([...used].sort()).toEqual([
    'map__full', 'map__full-ico', 'map__full-ico--in', 'map__full-ico--out',
    'map-zoom__btn', 'map-zoom__ico',
  ].sort())

  // map-zoom__ico carries no rule in the kit either — it is the kit's own
  // unstyled hook on the glyph, kept for parity rather than invented here, and
  // named as the exception so it cannot quietly grow company.
  for (const c of used) {
    if (c === 'map-zoom__ico') continue
    expect(css, `${c} missing from components.css`).toContain(`.${c}`)
  }
})

describe('orientation control', () => {
  const labels = {
    label: 'Orientation', tip: 'tip', tiltLabel: 'Tilt', headingLabel: 'Heading',
    northLabel: 'Reset to north', closeLabel: 'Close',
  }
  const fakeMap = () => {
    const listeners = {}
    return {
      bearing: 0,
      pitch: 0,
      setPitch: vi.fn(),
      setBearing: vi.fn(),
      easeTo: vi.fn(),
      getBearing() { return this.bearing },
      getPitch() { return this.pitch },
      getMaxPitch: () => 60,
      on: (name, fn) => { listeners[name] = fn },
      fire: (name) => listeners[name]?.(),
    }
  }
  const wire = (map = fakeMap()) => {
    const parts = mountOrientation(frame(), labels)
    installOrientation(map, parts)
    return { map, parts }
  }

  it('is a labelled button that starts collapsed, with the needle north-up', () => {
    const { parts } = wire()
    expect(parts.button.getAttribute('aria-label')).toBe('Orientation')
    expect(parts.button.getAttribute('aria-expanded')).toBe('false')
    expect(parts.panel.hidden).toBe(true)
    expect(parts.needle.getAttribute('transform')).toBe('rotate(0 8 8)')
    expect(parts.close.getAttribute('aria-label')).toBe('Close')
  })

  it('turns the needle against the bearing and syncs the sliders on rotate', () => {
    const { map, parts } = wire()
    map.bearing = 30
    map.pitch = 40
    map.fire('rotate')
    expect(parts.needle.getAttribute('transform')).toBe('rotate(-30 8 8)')
    expect(parts.heading.input.value).toBe('30')
    expect(parts.heading.out.textContent).toBe('30\u00b0')
    map.pitch = 20
    map.fire('pitch')
    expect(parts.tilt.input.value).toBe('20')
  })

  it('sizes the tilt slider from the map max pitch', () => {
    expect(wire().parts.tilt.input.max).toBe('60')
  })

  it('sends slider values to the map as the visitor\'s own moves', () => {
    const { map, parts } = wire()
    parts.tilt.input.value = '45'
    parts.tilt.input.dispatchEvent(new Event('input'))
    parts.heading.input.value = '-90'
    parts.heading.input.dispatchEvent(new Event('input'))
    expect(map.setPitch).toHaveBeenCalledWith(45, { userInitiated: true })
    expect(map.setBearing).toHaveBeenCalledWith(-90, { userInitiated: true })
  })

  it('eases to north-up flat without naming a centre or zoom', () => {
    const { map, parts } = wire()
    parts.north.click()
    expect(map.easeTo).toHaveBeenCalledWith({ pitch: 0, bearing: 0 }, { userInitiated: true })
  })

  it('opens on click, and Escape closes it and returns focus to the button', () => {
    const { parts } = wire()
    document.body.append(parts.el)
    parts.button.click()
    expect(parts.panel.hidden).toBe(false)
    expect(parts.button.getAttribute('aria-expanded')).toBe('true')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(parts.panel.hidden).toBe(true)
    expect(parts.button.getAttribute('aria-expanded')).toBe('false')
    expect(document.activeElement).toBe(parts.button)
  })

  it('closes on the X and on a click outside, but not on a click inside', () => {
    const { parts } = wire()
    document.body.append(parts.el)
    parts.button.click()
    parts.panel.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    expect(parts.panel.hidden).toBe(false)
    parts.close.click()
    expect(parts.panel.hidden).toBe(true)
    expect(document.activeElement).toBe(parts.button)
    parts.button.click()
    document.body.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    expect(parts.panel.hidden).toBe(true)
  })
})

describe('mountReset', () => {
  it('is an icon button named for a screen reader', () => {
    const el = frame()
    const b = mountReset(el, { label: 'Reset view' })
    expect(b.tagName).toBe('BUTTON')
    expect(b.type).toBe('button')
    expect(b.className).toBe('map-reset')
    expect(b.getAttribute('aria-label')).toBe('Reset view')
    expect(b.getAttribute('title')).toBe('Reset view')
    expect(b.querySelector('svg').getAttribute('aria-hidden')).toBe('true')
    expect(b.parentElement).toBe(el)
  })
})

describe('mountOrientation without a reset', () => {
  it('builds no reset button in the panel', () => {
    const parts = mountOrientation(frame(), { label: 'Orientation', northLabel: 'North', closeLabel: 'Close' })
    expect(parts.reset).toBeUndefined()
    expect(parts.panel.querySelector('[data-act="reset"]')).toBeNull()
  })
})
