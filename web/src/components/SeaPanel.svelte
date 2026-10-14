<script>
  // One bathing site as a compact card: the official class first, then the last lab samples
  // against the zone's limits, then the season strip. History and charts live in SeaDetails.
  import {
    seaName, fillSeason, fillSlots, creditDate, isRZISource, classLabel, chipLevel, latestSamples, sampleSources,
    formatDay, LEVEL_MARKS,
  } from '../lib/sea.js'
  import { seaInfoContent } from '../lib/infodialog.js'
  import SeaSwatch from './SeaSwatch.svelte'
  import SeaSeasonStrip from './SeaSeasonStrip.svelte'
  import SeaClassKey from './SeaClassKey.svelte'

  let { view, t, lang, colours, creditURL, onclose, oninfo = () => {}, ondetails = () => {} } = $props()

  const d = $derived(view.detail)
  const current = $derived(d?.classes?.[0] ?? null)
  const firstSeason = $derived(d?.classes?.length ? Math.min(...d.classes.map((c) => c.season)) : null)
  // Classes from the EEA annual dataset are marked.
  const fromDatahub = (c) => c?.source === 'datahub'
  const sourceText = (c) => fillSlots(t.classSourceDatahub, { year: c.season })

  // A sample shows only with its credit line, so an RZI row with no region name or no template is dropped.
  const chips = $derived(latestSamples(d?.samples, t, 3))
  const shownSources = $derived(sampleSources(chips, t))
  const hasRZI = $derived(chips.some((s) => isRZISource(s.source)))

  // Freshness is the API's flag on the newest RZI sample. Off season is the API's season_open flag,
  // and the newest RZI sample says which year the table holds.
  const latest = $derived(d?.latest ?? null)
  const offSeasonYear = $derived(d?.samples?.find((s) => isRZISource(s.source))?.season ?? null)
  const offSeasonText = $derived(d?.site?.rzi?.season_open === false && !latest && offSeasonYear !== null && t.latest.offSeason ? fillSlots(t.latest.offSeason, { year: offSeasonYear }) : '')
  const staleText = $derived(latest?.stale && t.latest.stale ? fillSlots(t.latest.stale, { date: creditDate(latest.date) }) : '')
  const freshText = $derived(staleText || offSeasonText)

  const levelText = (level) => (level === 2 ? t.overGood : level === 1 ? t.overExcellent : t.chips.keyWithin)

  const classInfo = $derived(seaInfoContent(t.info, d?.supplement?.url ?? ''))
  const samplesInfo = $derived({ title: t.chips.heading, paragraphs: [t.latest.singleSampleNote, t.info.rzi, hasRZI ? t.noticeUnofficial : ''], links: [] })
  const sourcesInfo = $derived({
    title: t.sourcesInfoLabel,
    paragraphs: [t.credit, hasRZI ? t.noticeUnofficial : ''],
    links: creditURL ? [{ label: t.info.linkEea, href: creditURL }] : [],
  })

  // The close button takes focus on open, so Escape and Tab start inside the card.
  let closeButton = $state()
  $effect(() => {
    if (view.id) closeButton?.focus({ preventScroll: true })
  })

  function onkeydown(e) {
    if (e.key === 'Escape') onclose()
  }
</script>

{#snippet infoButton(label, content)}
  <button type="button" class="scale__info sea-panel__info" aria-label={label} onclick={() => oninfo(content)}></button>
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

    <div class="sea-panel__badge">
      <SeaSwatch quality={current?.quality ?? null} {t} {colours} />
      <div class="sea-panel__badge-text">
        <strong>{classLabel(current?.quality, t)}</strong>
        {#if current}<span class="sea-panel__season">{fillSeason(t.officialClass, current.season)}</span>{/if}
        {#if fromDatahub(current)}<span class="sea-panel__source">{sourceText(current)}</span>{/if}
      </div>
      {@render infoButton(t.info.label, classInfo)}
    </div>

    <div class="sea-panel__h-row">
      <h3 class="sea-panel__h">{t.chips.heading}</h3>
      {@render infoButton(t.chips.infoLabel, samplesInfo)}
    </div>
    {#if chips.length}
      <ul class="sea-panel__chips">
        {#each chips as s, i (`${s.date}-${s.source ?? ''}-${i}`)}
          {@const level = chipLevel(s, d.limits)}
          <li class="sea-chip">
            <span class="sea-chip__date">{formatDay(s.date, lang)}</span>
            <span class="sea-chip__mark" aria-hidden="true">{LEVEL_MARKS[level]}</span>
            <span class="sr-only">, {levelText(level)}</span>
            {#if s.pre_season}<span class="sea-chip__pre">{t.preSeason}</span>{/if}
          </li>
        {/each}
      </ul>
      <p class="sea-panel__key"><span aria-hidden="true">{LEVEL_MARKS[0]}</span> {t.chips.keyWithin} · <span aria-hidden="true">{LEVEL_MARKS[1]}</span> {t.overExcellent} · <span aria-hidden="true">{LEVEL_MARKS[2]}</span> {t.overGood}</p>
    {:else}
      <p class="sea-panel__msg">{t.chips.none}</p>
    {/if}
    {#if freshText}<p class="sea-panel__fresh">{freshText}</p>{/if}

    {#if firstSeason !== null}
      <h3 class="sea-panel__h">{fillSlots(t.seasonsSince, { first: firstSeason })}</h3>
      <SeaSeasonStrip classes={d.classes} {t} {colours} variant="strip" />
      <SeaClassKey {t} {colours} />
    {/if}

    <button type="button" class="sea-panel__details" onclick={(e) => ondetails(e.currentTarget)}>{t.details}</button>

    <p class="sea-panel__sources">
      <span>{shownSources.map((s) => s.name).join(', ')}</span>
      {@render infoButton(t.sourcesInfoLabel, sourcesInfo)}
    </p>
  {:else}
    <p class="sea-panel__msg" aria-busy="true">…</p>
  {/if}
</section>
