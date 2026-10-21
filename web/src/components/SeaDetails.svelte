<script>
  // The details view of one bathing site, in a native dialog: samples with a chart, the season
  // history, and what the data is. Tab, indicator and season are kept while this component lives.
  // SeaPanel unmounts it while another site loads (its detail is null then), and the {#key} on the
  // site id remounts it for a direct switch, so either way they reset.
  import {
    seaName, fillSlots, formatDay, safePageURL, isRZISource, remarkedSamples, rhiName, nextTab, pendingSeason, seaLevel, rziCredit, creditDate,
    creditedSamples, seasonsWithSamples, samplePoints, sampleSources, SEA_METRICS, levelText, httpURL, offSeasonText,
  } from '../lib/sea.js'
  import SeaSampleChart from './SeaSampleChart.svelte'
  import SeaSeasonStrip from './SeaSeasonStrip.svelte'
  import SeaClassKey from './SeaClassKey.svelte'

  let { d, t, lang, colours, creditURL } = $props()

  const TABS = ['samples', 'seasons', 'about']
  const MARKS = ['', '▲', '▲▲']
  let tab = $state('samples')
  let metric = $state(SEA_METRICS[0])
  let chosenSeason = $state(null)
  let dialog = $state()
  let tabEls = $state([])
  let opener = null

  const samples = $derived(creditedSamples(d.samples, t))
  const remarked = $derived(remarkedSamples(samples, t).sort((a, b) => (a.date < b.date ? 1 : a.date > b.date ? -1 : 0)))
  const seasons = $derived(seasonsWithSamples(samples))
  const season = $derived(seasons.includes(chosenSeason) ? chosenSeason : (seasons[0] ?? null))
  const points = $derived(season === null ? [] : samplePoints(samples, season, metric))
  const metricName = $derived(metric === 'enterococci' ? t.enterococci : t.eColi)
  const allSources = $derived(sampleSources(samples, t))
  // With a credit link the EEA line is that link, so it is not listed again.
  const listedSources = $derived(allSources.filter((s) => !(creditURL && s.id === 'eea')))
  const pending = $derived(pendingSeason({ classes: d.classes, samples: d.samples }))
  const firstSeason = $derived(d.classes?.length ? Math.min(...d.classes.map((c) => c.season)) : null)
  const datahubClasses = $derived((d.classes ?? []).filter((c) => c.source === 'datahub'))
  const hasRZI = $derived(!!d.site?.rzi || samples.some((s) => isRZISource(s.source)))
  const rziPage = $derived(safePageURL(d.site?.rzi?.page_url))
  // The profile is often a PDF, so only the scheme is checked.
  const profileURL = $derived(httpURL(d.site?.profile_url))

  // The latest RZI result, as the card showed it before the redesign. It shows only with something
  // credited or the closed-season note, never as a bare heading.
  const latest = $derived(d.latest ?? null)
  const latestCredit = $derived(latest ? rziCredit(t, latest.source, latest.date) : '')
  const offSeasonNote = $derived(offSeasonText(t, d, samples))
  const staleText = $derived(latest?.stale && t.latest.stale ? fillSlots(t.latest.stale, { date: creditDate(latest.date) }) : '')
  const hasLab = $derived(latest ? !!latestCredit && (!latest.stale || !!staleText) : !!offSeasonNote)

  export function show(from) {
    opener = from ?? null
    if (!dialog.open) dialog.showModal()
  }

  // Native close: Escape, Back or the close button. Focus goes back to where the dialog was opened from.
  function onclose() {
    opener?.focus()
    opener = null
  }

  // The dialog sits inside the card, so an Escape here would also reach the card's own handler and close it.
  function stopEscape(e) {
    if (e.key === 'Escape') e.stopPropagation()
  }

  function onTabKey(e) {
    const next = nextTab(TABS.indexOf(tab), e.key, TABS.length)
    if (next === null) return
    e.preventDefault()
    tab = TABS[next]
    tabEls[next]?.focus()
  }
</script>

{#snippet newTab()}{#if t.newTab}<span class="sr-only">, {t.newTab}</span>{/if}{/snippet}

{#snippet reading(value, below, limits)}
  {@const level = seaLevel(value, below, limits)}
  {below ? '<' : ''}{value}{#if level}<span aria-hidden="true"> {MARKS[level]}</span>{/if}
  {#if below}<span class="sr-only">, {t.belowDetection}</span>{/if}
  {#if level}<span class="sr-only">, {levelText(t, level)}</span>{/if}
{/snippet}

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<dialog class="sea-details" bind:this={dialog} aria-labelledby="sea-details-title" {onclose} onkeydown={stopEscape}>
  <header class="sea-details__head">
    <button type="button" class="sea-details__back" onclick={() => dialog.close()}>{t.back}</button>
    <h2 class="sea-details__title" id="sea-details-title">{seaName(d.site, lang)}</h2>
    <button type="button" class="sea-details__close" aria-label={t.close} onclick={() => dialog.close()}>
      <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true" focusable="false"><path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" /></svg>
    </button>
  </header>

  <div class="sea-details__tabs" role="tablist" aria-label={t.tabs.label}>
    {#each TABS as id, i (id)}
      <button
        type="button" role="tab" id={`sea-tab-${id}`} aria-selected={tab === id} aria-controls={`sea-panel-${id}`}
        tabindex={tab === id ? 0 : -1} bind:this={tabEls[i]} onclick={() => (tab = id)} onkeydown={onTabKey}
      >{t.tabs[id]}</button>
    {/each}
  </div>

  <div class="sea-details__body">
    <div role="tabpanel" id="sea-panel-samples" aria-labelledby="sea-tab-samples" tabindex="0" hidden={tab !== 'samples'}>
      {#if pending}<p class="sea-details__pending">{fillSlots(t.samplesPending, { season: pending })}</p>{/if}
      {#if seasons.length === 0}
        <p class="sea-details__msg">{t.noSamples}</p>
      {:else}
        <div class="sea-details__controls">
          <div class="sea-seg" role="group" aria-label={t.metricLabel}>
            {#each SEA_METRICS as m (m)}
              <button type="button" class="sea-seg__btn" aria-pressed={metric === m} onclick={() => (metric = m)}>{m === 'enterococci' ? t.enterococci : t.eColi}</button>
            {/each}
          </div>
          {#if seasons.length > 1}
            <label class="sea-details__season"><span>{t.seasonPicker}</span>
              <select class="sea-details__select" onchange={(e) => (chosenSeason = Number(e.currentTarget.value))}>
                {#each seasons as s (s)}<option value={s} selected={s === season}>{s}</option>{/each}
              </select>
            </label>
          {/if}
        </div>
        <SeaSampleChart {points} limits={d.limits[metric]} {metricName} {season} {t} {lang} />
      {/if}
      {#if remarked.length}
        <ul class="sea-remarks">
          {#each remarked as s, i (`${s.date}-${s.source}-${i}`)}
            <li class="sea-remark">
              <div role="group" aria-labelledby={`sea-details-remark-${i}`}>
                <p class="sea-remark__label" id={`sea-details-remark-${i}`}>{t.remark.label}, {formatDay(s.date, lang)}</p>
                <p class="sea-remark__text">{s.remarks}</p>
                <p class="sea-remark__credit">{fillSlots(t.remark.credit, { rhi: rhiName(t, s.source) })}</p>
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <div role="tabpanel" id="sea-panel-seasons" aria-labelledby="sea-tab-seasons" tabindex="0" hidden={tab !== 'seasons'}>
      {#if firstSeason !== null}
        <h3 class="sea-details__h">{fillSlots(t.seasonsSince, { first: firstSeason })}</h3>
        <SeaSeasonStrip classes={d.classes} {t} {colours} variant="grid" />
        <SeaClassKey {t} {colours} />
        {#each datahubClasses as c (c.season)}
          <p class="sea-details__source">{c.season}: {fillSlots(t.classSourceDatahub, { year: c.season })}</p>
        {/each}
        {#if d.supplement}
          <p class="sea-details__supplement">{fillSlots(t.supplementNote, { edition: d.supplement.edition, published: formatDay(d.supplement.published, lang) })}</p>
        {/if}
      {:else}
        <p class="sea-details__msg">{t.classes.not_classified}</p>
      {/if}
    </div>

    <div role="tabpanel" id="sea-panel-about" aria-labelledby="sea-tab-about" tabindex="0" hidden={tab !== 'about'}>
      <p>{t.info.body}</p>
      <p>{t.note}</p>
      <h3 class="sea-details__h">{t.limits} ({t.unit})</h3>
      <p>{t.eColi} {d.limits.e_coli[0]} / {d.limits.e_coli[1]}, {t.enterococci} {d.limits.enterococci[0]} / {d.limits.enterococci[1]}</p>
      {#if hasLab}
        <div class="sea-details__latest">
          <h3 class="sea-details__h">{t.latest.heading}</h3>
          {#if !latest}
            <p class="sea-details__off-season">{offSeasonNote}</p>
          {:else if latest.stale}
            <p class="sea-details__stale">{staleText}</p>
            <p class="sea-details__credit-rzi">{latestCredit}</p>
          {:else}
            <p class="sea-details__latest-values">
              <span class="sea-details__latest-value">{t.eColi}: {@render reading(latest.e_coli, latest.e_coli_below_detection, d.limits.e_coli)}<span class="sr-only"> {t.unitMpn}</span></span>
              <span class="sea-details__latest-value">{t.enterococci}: {@render reading(latest.enterococci, latest.enterococci_below_detection, d.limits.enterococci)}<span class="sr-only"> {t.unitMpn}</span></span>
              <span class="sea-details__latest-unit" aria-hidden="true">{t.unitMpn}</span>
            </p>
            <p class="sea-details__credit-rzi">{latestCredit}</p>
            {#if t.latest.singleSampleNote}<p class="sea-details__latest-note">{t.latest.singleSampleNote}</p>{/if}
          {/if}
        </div>
      {/if}
      {#if hasRZI}
        <p>{t.info.rzi}</p>
        <p class="sea-details__notice">{t.noticeUnofficial}</p>
        {#if rziPage}<p><a href={rziPage} rel="noopener noreferrer" target="_blank">{t.sourceLink}{@render newTab()}</a></p>{/if}
      {/if}
      {#if listedSources.length}
        <ul class="sea-details__credits">
          {#each listedSources as s (s.id)}<li>{s.credit}</li>{/each}
        </ul>
      {/if}
      <p class="sea-details__links">
        {#if creditURL}<a href={creditURL} rel="noopener noreferrer" target="_blank">{t.credit}{@render newTab()}</a>{/if}
        {#if profileURL}{#if creditURL}· {/if}<a href={profileURL} rel="noopener noreferrer" target="_blank">{t.profile}{@render newTab()}</a>{/if}
      </p>
    </div>
  </div>
</dialog>
