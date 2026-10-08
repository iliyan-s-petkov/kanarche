// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { createInfoDialog, pollenInfoContent, seaInfoContent, POLLEN_LINKS, SEA_LINKS } from '../infodialog.js'

// jsdom has no showModal, so each dialog gets a stand-in that flips `open`.
function dialog() {
  const dlg = createInfoDialog(document, { closeLabel: 'Close' })
  dlg.el.showModal = vi.fn(function () { this.open = true })
  dlg.el.close = vi.fn(function () { this.open = false })
  return dlg
}

describe('createInfoDialog', () => {
  const content = {
    title: 'Pollen forecast',
    paragraphs: ['First.', 'Second.'],
    links: [{ label: 'One', href: 'https://example.org/one' }, { label: 'Two', href: 'https://example.org/two' }],
  }

  it('paints the title, paragraphs and links, and opens modally', () => {
    const dlg = dialog()
    dlg.show(content)
    expect(dlg.el.showModal).toHaveBeenCalledTimes(1)
    expect(dlg.el.querySelector('h2').textContent).toBe('Pollen forecast')
    expect([...dlg.el.querySelectorAll('p')].map((p) => p.textContent)).toEqual(['First.', 'Second.'])
    const links = [...dlg.el.querySelectorAll('a')]
    expect(links.map((a) => a.textContent)).toEqual(['One', 'Two'])
    expect(links.map((a) => a.getAttribute('href'))).toEqual(['https://example.org/one', 'https://example.org/two'])
  })

  it('opens every link in a new tab with rel noopener', () => {
    const dlg = dialog()
    dlg.show(content)
    for (const a of dlg.el.querySelectorAll('a')) {
      expect(a.target).toBe('_blank')
      expect(a.rel).toContain('noopener')
    }
  })

  it('repaints on a second show and does not reopen an open dialog', () => {
    const dlg = dialog()
    dlg.show(content)
    dlg.show({ title: 'Sea', paragraphs: ['Only.'], links: [] })
    expect(dlg.el.showModal).toHaveBeenCalledTimes(1)
    expect(dlg.el.querySelectorAll('p')).toHaveLength(1)
    expect(dlg.el.querySelectorAll('a')).toHaveLength(0)
  })

  it('has a labelled close button that closes it', () => {
    const dlg = dialog()
    dlg.show(content)
    const close = [...dlg.el.querySelectorAll('button')].find((b) => b.textContent === 'Close')
    close.click()
    expect(dlg.el.close).toHaveBeenCalled()
  })
})

describe('pollenInfoContent', () => {
  it('orders body then hourly, and links the thresholds then the CAMS chart', () => {
    const c = pollenInfoContent({ title: 'T', body: 'B', hourly: 'H', linkThresholds: 'Limits', linkChart: 'Chart' })
    expect(c.title).toBe('T')
    expect(c.paragraphs).toEqual(['B', 'H'])
    expect(c.links).toEqual([
      { label: 'Limits', href: 'https://climate-adapt.eea.europa.eu/en/observatory/publications-data/analysis-data/cams-ground-level-pollen-forecast' },
      { label: 'Chart', href: 'https://atmosphere.copernicus.eu/charts/packages/cams_air_quality/products/europe-air-quality-forecast-pollens' },
    ])
    expect(POLLEN_LINKS.chart).toBe(c.links[1].href)
  })
})

describe('seaInfoContent', () => {
  it('links the EEA map then the EEA bathing water page', () => {
    const c = seaInfoContent({ title: 'T', body: 'B', linkMap: 'Map', linkEea: 'EEA' })
    expect(c.paragraphs).toEqual(['B'])
    expect(c.links).toEqual([
      { label: 'Map', href: 'https://www.eea.europa.eu/en/analysis/maps-and-charts/state-of-bathing-waters-in-2025' },
      { label: 'EEA', href: 'https://www.eea.europa.eu/en/topics/in-depth/bathing-water' },
    ])
    expect(SEA_LINKS.eea).toBe(c.links[1].href)
  })

  const info = { title: 'T', body: 'B', linkMap: 'Map', linkEea: 'EEA', linkDatahub: 'Dataset' }
  it('adds the Datahub dataset link from the API url', () => {
    const c = seaInfoContent(info, 'https://www.eea.europa.eu/en/datahub/datahubitem-view/x')
    expect(c.links.at(-1)).toEqual({ label: 'Dataset', href: 'https://www.eea.europa.eu/en/datahub/datahubitem-view/x' })
    expect(c.links).toHaveLength(3)
  })
  it('has no Datahub link without a supplement url', () => {
    for (const url of [undefined, null, '']) expect(seaInfoContent(info, url).links).toHaveLength(2)
  })
  it('never links a url that is not https', () => {
    expect(seaInfoContent(info, 'javascript:alert(1)').links).toHaveLength(2)
    expect(seaInfoContent(info, 'http://example.org/a').links).toHaveLength(2)
  })
})
