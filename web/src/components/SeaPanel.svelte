<script>
  // One bathing site: the EEA's annual class first, then the samples against the zone's limits.
  import { seaClass, seaLevel, seaName, fillSeason, fillSlots } from '../lib/sea.js'

  let { view, t, lang, colours, creditURL, onclose } = $props()

  const d = $derived(view.detail)
  const current = $derived(d?.classes?.[0] ?? null)
  const earlier = $derived(d?.classes?.slice(1) ?? [])
  const dateFmt = $derived(new Intl.DateTimeFormat(lang || 'bg', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' }))

  const qualityLabel = (q) => t.classes[q ?? 'not_classified'] || t.classes.not_classified || q || ''
  const formatDate = (iso) => dateFmt.format(new Date(`${iso}T00:00:00Z`))
  // Classes from the EEA annual dataset are marked. Their lab samples arrive later, so the
  // note shows while no sample of that season is listed.
  const fromDatahub = (c) => c?.source === 'datahub'
  const sourceText = (c) => fillSlots(t.classSourceDatahub, { year: c.season })
  const samplesPending = $derived(fromDatahub(current) && !d?.samples?.some((s) => s.season === current.season))
  const supplement = $derived(d?.supplement ?? null)
  // The edition's publish date is an ISO day; anything else is shown as sent.
  const publishedText = (iso) => (/^\d{4}-\d{2}-\d{2}$/.test(iso) ? formatDate(iso) : iso)
  const MARKS = ['', '▲', '▲▲']
  const levelText = (level) => (level === 2 ? t.overGood : level === 1 ? t.overExcellent : '')

  // The close button takes focus on open, so Escape and Tab start inside the card.
  let closeButton = $state()
  $effect(() => {
    if (view.id) closeButton?.focus({ preventScroll: true })
  })

  function onkeydown(e) {
    if (e.key === 'Escape') onclose()
  }
</script>

{#snippet swatch(colour)}
  <svg class="sea-panel__swatch" viewBox="0 0 1 1" aria-hidden="true" focusable="false"><rect width="1" height="1" fill={colour} /></svg>
{/snippet}

{#snippet reading(value, below, limits)}
  {@const level = seaLevel(value, below, limits)}
  {below ? '<' : ''}{value}{#if level}<span aria-hidden="true" class="sea-panel__mark"> {MARKS[level]}</span>{/if}
  {#if below}<span class="sr-only">, {t.belowDetection}</span>{/if}
  {#if level}<span class="sr-only">, {levelText(level)}</span>{/if}
{/snippet}

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<section class="sea-panel" aria-labelledby="sea-panel-title" {onkeydown}>
  <header class="sea-panel__head">
    <h2 class="sea-panel__title" id="sea-panel-title">{d ? seaName(d.site, lang) : ''}</h2>
    <button type="button" class="sea-panel__close" aria-label={t.close} bind:this={closeButton} onclick={onclose}>
      <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true" focusable="false"><path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" /></svg>
    </button>
  </header>

  {#if view.failed}
    <p class="sea-panel__msg">{t.failed}</p>
  {:else if d}
    <p class="sea-panel__zone">{t.zones[d.site.zone] || ''}</p>
    <p class="sea-panel__class">
      {@render swatch(colours[seaClass(current?.quality ?? null)])}
      <strong>{qualityLabel(current?.quality)}</strong>
      {#if current}<span class="sea-panel__season">{fillSeason(t.season, current.season)}</span>{/if}
      {#if fromDatahub(current)}<span class="sea-panel__source">{sourceText(current)}</span>{/if}
    </p>
    {#if earlier.length}
      <h3 class="sea-panel__h">{t.history}</h3>
      <ul class="sea-panel__history">
        {#each earlier as c (c.season)}
          <li>{@render swatch(colours[seaClass(c.quality)])}{c.season}: {qualityLabel(c.quality)}{#if fromDatahub(c)} <span class="sea-panel__source">{sourceText(c)}</span>{/if}</li>
        {/each}
      </ul>
    {/if}
    {#if supplement}
      <p class="sea-panel__supplement">{fillSlots(t.supplementNote, { edition: supplement.edition, published: publishedText(supplement.published) })}</p>
    {/if}

    <h3 class="sea-panel__h">{t.samples}</h3>
    {#if samplesPending}<p class="sea-panel__pending">{fillSlots(t.samplesPending, { season: current.season })}</p>{/if}
    <p class="sea-panel__limits">
      {t.limits} ({t.unit}): {t.eColi} {d.limits.e_coli[0]} / {d.limits.e_coli[1]}, {t.enterococci} {d.limits.enterococci[0]} / {d.limits.enterococci[1]}
    </p>
    {#if d.samples.length}
      <div class="sea-panel__scroll">
        <table class="sea-panel__samples">
          <thead><tr><th scope="col">{t.date}</th><th scope="col">{t.eColi}</th><th scope="col">{t.enterococci}</th></tr></thead>
          <tbody>
            {#each d.samples as s (s.date)}
              <tr>
                <th scope="row">{formatDate(s.date)}{#if s.pre_season} <span class="sea-panel__pre">{t.preSeason}</span>{/if}</th>
                <td>{@render reading(s.e_coli, s.e_coli_below_detection, d.limits.e_coli)}</td>
                <td>{@render reading(s.enterococci, s.enterococci_below_detection, d.limits.enterococci)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      <p class="sea-panel__key"><span aria-hidden="true">▲</span> {t.overExcellent} · <span aria-hidden="true">▲▲</span> {t.overGood}</p>
    {:else}
      <p class="sea-panel__msg">{t.noSamples}</p>
    {/if}

    <p class="sea-panel__note">{t.note}</p>
    <p class="sea-panel__credit">
      <a href={creditURL} rel="noopener noreferrer" target="_blank">{t.credit}</a>
      {#if d.site.profile_url}· <a href={d.site.profile_url} rel="noopener noreferrer" target="_blank">{t.profile}</a>{/if}
    </p>
  {:else}
    <p class="sea-panel__msg" aria-busy="true">…</p>
  {/if}
</section>
