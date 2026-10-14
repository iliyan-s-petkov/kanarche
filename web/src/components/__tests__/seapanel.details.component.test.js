// @vitest-environment jsdom
import { describe, it, expect, afterEach, beforeAll } from 'vitest'
import { flushSync } from 'svelte'
import { createSeaPanel } from '../../lib/seapanel.svelte.js'
import { seaT, colours, detail } from './seafixtures.js'

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function showModal() { this.open = true }
  HTMLDialogElement.prototype.close = function close() { this.open = false; this.dispatchEvent(new Event('close')) }
})

const cfg = { lang: 'en', seaColours: colours, seaCreditURL: 'https://www.eea.europa.eu/', t: { sea: seaT } }
const second = { ...detail, site: { ...detail.site, id: 'BG2', name_en: 'SECOND' } }
let frame
afterEach(() => { frame?.remove() })

async function opened(id = 'BG1') {
  frame = document.createElement('div')
  document.body.appendChild(frame)
  const panel = createSeaPanel(frame, cfg, async (url) => (url.endsWith('BG2') ? second : detail))
  await panel.open(id)
  flushSync()
  return panel
}
const tab = (id) => frame.querySelector(`#sea-tab-${id}`)

describe('card and details together', () => {
  it('opens the details dialog from the Details button', async () => {
    await opened()
    const dialog = frame.querySelector('dialog.sea-details')
    expect(dialog.open).toBe(false)
    frame.querySelector('button.sea-panel__details').click()
    flushSync()
    expect(dialog.open).toBe(true)
  })

  it('returns focus to the Details button when the dialog closes', async () => {
    await opened()
    const btn = frame.querySelector('button.sea-panel__details')
    btn.click()
    frame.querySelector('.sea-details__back').click()
    expect(document.activeElement).toBe(btn)
  })

  // Review Focus 5.
  it('closes only the dialog on Escape, never the card behind it', async () => {
    const panel = await opened()
    frame.querySelector('button.sea-panel__details').click()
    frame.querySelector('.sea-details__title').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    flushSync()
    expect(frame.querySelector('.map-sea').hidden).toBe(false)
    expect(panel.id).toBe('BG1')
  })

  it('resets the tab when another site opens', async () => {
    const panel = await opened()
    tab('seasons').click()
    flushSync()
    expect(tab('seasons').getAttribute('aria-selected')).toBe('true')
    await panel.open('BG2')
    flushSync()
    expect(tab('samples').getAttribute('aria-selected')).toBe('true')
  })

  it('keeps the tab while the same site stays open', async () => {
    await opened()
    tab('about').click()
    flushSync()
    frame.querySelector('.sea-details__back').click()
    frame.querySelector('button.sea-panel__details').click()
    expect(tab('about').getAttribute('aria-selected')).toBe('true')
  })
})
