<script>
  // Every sensor in this area as a table: the map's text alternative and the keyboard route to each sensor.
  // Rows come from the registry the map island fills, so the table costs no request and agrees with the map.
  import { getSensors, getSensorArea, getScales } from '../lib/sensors.svelte.js'
  import { bandsFor } from '../lib/mappaint.js'
  import { number } from '../lib/areacards.js'
  import { formatLocalStamp } from '../lib/localtime.js'
  import {
    METRICS, PER_PAGE_OPTIONS, DEFAULT_PER_PAGE,
    sensorRows, filterSensorRows, pageView, statusText, summaryText, fill,
  } from '../lib/areasensors.js'

  let { texts, names, area, slug, districts = {}, lang = 'bg', onopen, onready } = $props()

  let query = $state('')
  let perPage = $state(DEFAULT_PER_PAGE)
  let page = $state(1)

  // The last body loaded for this page's own area. A slug the map adopted elsewhere does not replace it.
  let own = null
  const body = $derived.by(() => {
    const b = getSensors()
    if (getSensorArea() === slug) own = b
    return own
  })
  $effect(() => { if (body) onready?.() })

  const all = $derived(sensorRows(body, {
    districts,
    bands: { P2: bandsFor(getScales(), 'P2'), P1: bandsFor(getScales(), 'P1') },
    names,
  }))
  const view = $derived(pageView(filterSensorRows(all, query), perPage, page))
  const hasDistricts = $derived(Object.keys(districts).length > 0)
  // Sensor, band and updated, plus one per metric and the district when shown.
  const columns = $derived(3 + METRICS.length + (hasDistricts ? 1 : 0))

  function setQuery(next) {
    query = next
    page = 1
  }

  function setPerPage(value) {
    perPage = value === 'all' ? 'all' : Number(value)
    page = 1
  }

  function go(where) {
    if (where === 'first') page = 1
    else if (where === 'prev') page = Math.max(1, view.page - 1)
    else if (where === 'next') page = Math.min(view.pages, view.page + 1)
    else page = view.pages
  }

  const reading = (v) => (typeof v === 'number' ? number(v, lang) : texts.noValue)
</script>

{#if body}
<details class="area-sensors">
  <summary>{summaryText(texts, all.length)}</summary>

  <div class="area-sensors__controls">
    <div class="field field--search">
      <label class="field__label" for="area-sensors-filter">{texts.filter}</label>
      <input
        class="input"
        id="area-sensors-filter"
        type="search"
        inputmode="numeric"
        autocomplete="off"
        aria-describedby="area-sensors-filter-hint"
        placeholder={texts.filterPlaceholder}
        value={query}
        oninput={(e) => setQuery(e.currentTarget.value)}
      >
      <span class="sr-only" id="area-sensors-filter-hint">{texts.filterHint}</span>
    </div>
    <p class="meta area-sensors__status" role="status">{statusText(texts, view, all.length)}</p>
  </div>

  <div class="area-sensors__scroll">
    <table class="table area-sensors__table">
      <caption>{fill(texts.caption, { area })}</caption>
      <thead>
        <tr>
          <th scope="col">{texts.colSensor}</th>
          {#if hasDistricts}<th scope="col">{texts.colDistrict}</th>{/if}
          {#each METRICS as m (m)}
            <th scope="col" class="num">{fill(texts.colValue, { metric: texts.metrics[m], unit: texts.units[m] })}</th>
          {/each}
          <th scope="col">{texts.colBand}</th>
          <th scope="col">{texts.colUpdated}</th>
        </tr>
      </thead>
      <tbody>
        {#each view.rows as row (row.id)}
          <tr>
            <th scope="row">
              <!-- Focus scrolls the row into view, so a sensor closed from the panel returns to a visible button. -->
              <button
                type="button"
                class="btn btn--ghost area-sensors__open"
                aria-label={fill(texts.open, { id: row.id })}
                onclick={() => onopen(row)}
                onfocus={(e) => e.currentTarget.scrollIntoView?.({ block: 'nearest' })}
              >{row.id}</button>
            </th>
            {#if hasDistricts}<td>{row.district}</td>{/if}
            {#each METRICS as m (m)}<td class="num">{reading(row[m])}</td>{/each}
            <td>{row.band}</td>
            <td>{#if row.updated}<time datetime={row.updated}>{formatLocalStamp(row.updated, lang)}</time>{/if}</td>
          </tr>
        {:else}
          <tr><td colspan={columns}>{statusText(texts, view, all.length)}</td></tr>
        {/each}
      </tbody>
    </table>
  </div>

  <nav class="pager" aria-label={texts.pagerLabel}>
    <div class="pager__perpage">
      <label for="area-sensors-perpage">{texts.perPage}</label>
      <span class="select-wrap">
        <select class="select" id="area-sensors-perpage" value={String(perPage)} onchange={(e) => setPerPage(e.currentTarget.value)}>
          {#each PER_PAGE_OPTIONS as n (n)}<option value={String(n)}>{n}</option>{/each}
          <option value="all">{texts.perPageAll}</option>
        </select>
        <span class="select-wrap__caret" aria-hidden="true"></span>
      </span>
    </div>
    <span class="pager__spacer"></span>
    {#if view.pages > 1}
      <div class="pager__nav">
        <div class="pager__group">
          <button type="button" class="btn btn--ghost pager__btn" aria-label={texts.pageFirst}
                  disabled={view.page === 1} onclick={() => go('first')}>&lsaquo;&lsaquo;</button>
          <button type="button" class="btn btn--ghost pager__btn" aria-label={texts.pagePrev}
                  disabled={view.page === 1} onclick={() => go('prev')}>&lsaquo;</button>
        </div>
        <p class="pager__status">{texts.page} {view.page} {texts.pageOf} {view.pages}</p>
        <div class="pager__group">
          <button type="button" class="btn btn--ghost pager__btn" aria-label={texts.pageNext}
                  disabled={view.page === view.pages} onclick={() => go('next')}>&rsaquo;</button>
          <button type="button" class="btn btn--ghost pager__btn" aria-label={texts.pageLast}
                  disabled={view.page === view.pages} onclick={() => go('last')}>&rsaquo;&rsaquo;</button>
        </div>
      </div>
    {/if}
  </nav>
</details>
{/if}
