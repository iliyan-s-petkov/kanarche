// The (i) dialogs for the pollen and bathing-water legend sections: a title,
// some paragraphs and outbound links. Same native <dialog> and classes as the
// metric scale dialog (scaledialog.js), so Escape, focus trap and focus return
// are browser behaviour here too.

export const POLLEN_LINKS = {
  thresholds: 'https://climate-adapt.eea.europa.eu/en/observatory/publications-data/analysis-data/cams-ground-level-pollen-forecast',
  chart: 'https://atmosphere.copernicus.eu/charts/packages/cams_air_quality/products/europe-air-quality-forecast-pollens',
}

export const SEA_LINKS = {
  map: 'https://www.eea.europa.eu/en/analysis/maps-and-charts/state-of-bathing-waters-in-2025',
  eea: 'https://www.eea.europa.eu/en/topics/in-depth/bathing-water',
}

// Dialog content from the data-t-pollen-info-* texts.
export function pollenInfoContent(info) {
  return {
    title: info.title,
    paragraphs: [info.body, info.hourly],
    links: [
      { label: info.linkThresholds, href: POLLEN_LINKS.thresholds },
      { label: info.linkChart, href: POLLEN_LINKS.chart },
    ],
  }
}

// Dialog content from the data-t-sea-info-* texts.
export function seaInfoContent(info) {
  return {
    title: info.title,
    paragraphs: [info.body],
    links: [
      { label: info.linkMap, href: SEA_LINKS.map },
      { label: info.linkEea, href: SEA_LINKS.eea },
    ],
  }
}

// Built once; `show` repaints it from a content object and opens it.
export function createInfoDialog(doc, { closeLabel }) {
  const el = doc.createElement('dialog')
  el.className = 'scaleinfo'

  const dismiss = doc.createElement('button')
  dismiss.type = 'button'
  dismiss.className = 'scaleinfo__dismiss'
  dismiss.setAttribute('aria-label', closeLabel || '')
  dismiss.addEventListener('click', () => el.close())

  const heading = doc.createElement('h2')
  heading.className = 'scaleinfo__title'

  const body = doc.createElement('div')

  const close = doc.createElement('button')
  close.type = 'button'
  close.className = 'scaleinfo__close'
  close.textContent = closeLabel || ''
  close.addEventListener('click', () => el.close())

  el.append(dismiss, heading, body, close)

  return {
    el,
    show({ title, paragraphs, links }) {
      heading.textContent = title
      const nodes = paragraphs.filter(Boolean).map((text) => {
        const p = doc.createElement('p')
        p.className = 'scaleinfo__notes'
        p.textContent = text
        return p
      })
      for (const { label, href } of links) {
        const a = doc.createElement('a')
        a.className = 'link scaleinfo__source'
        a.textContent = label
        a.href = href
        // Outbound: the opener stays unreachable from the other site.
        a.target = '_blank'
        a.rel = 'noopener noreferrer'
        nodes.push(a)
      }
      body.replaceChildren(...nodes)
      if (!el.open) el.showModal()
    },
  }
}
