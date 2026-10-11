<script>
  // sensor and chart are documented in the task-8 brief's Interfaces block,
  // but that list predates the component: the actual props this component
  // reads are rows/title/flagText/closeLabel/noValue/onclose/chart. `sensor`
  // is not needed here — panelRows already reduced the sensor to `rows`
  // before this component ever runs, and the caller composes `title` from
  // the (non-templated) i18n label plus the sensor id. `chart` stays because
  // Step 7's own code renders it as an optional snippet.
  //
  // `open` is task 9's addition: the panel is mounted once, for the page's
  // lifetime, by islands/panel.js — whether it is SHOWING is a function of
  // whether a sensor is currently selected (viewstate's sensorId resolved
  // through the registry), which changes many times without a remount.
  // Defaults true so this component's own tests (which never pass it) are
  // unaffected: they are about what renders once shown, not about the
  // showing/hiding decision itself, which belongs to the caller that knows
  // about sensorId and the registry.
  //
  // `details` (added with the panel's chart controls) describes the station
  // rather than its readings — hardware, lifetime, coordinates. Defaults to
  // empty so a caller that has nothing to say renders no second list.
  // `meta` (task 12) is the station's EEA classification (EoI code, type,
  // area) — precomposed rows like `details`, but shown directly under the
  // heading rather than behind a disclosure: unlike the hardware inventory,
  // this is what the station IS, not a fact a reader has to open a drawer
  // for. `network` is a single precomposed sentence, the same idiom as
  // `flagText`. Both default empty so every sensor without them (every
  // citizen device today) renders neither block.
  import Gauge from './Gauge.svelte'
  import AboutSheet from './AboutSheet.svelte'
  import { createPanelLink } from '../lib/panellink.svelte.js'
  import { relativeAge } from '../lib/relativeage.js'
  import { number } from '../lib/areacards.js'

  // `source` and `updated` (ms) feed the phone header's "network · N min ago";
  // `link` is shared with the chart so a gauge tap picks its metric and the
  // chart menu can open the about sheet. `now` pins the clock for tests.
  let {
    rows, title, flagText, closeLabel, noValue, onclose,
    details = [], detailsLabel = '', chart = null, open = true,
    meta = [], network = '',
    favourite = false, favSave = '', favRemove = '', onstar = null,
    source = '', updated = null, now = null, locale = undefined,
    link = createPanelLink(),
  } = $props()

  let clock = $state(Date.now())
  $effect(() => {
    if (now !== null) return
    const id = setInterval(() => { clock = Date.now() }, 30000)
    return () => clearInterval(id)
  })

  const subline = $derived(
    [source, updated === null ? '' : relativeAge(updated, now ?? clock, locale)]
      .filter(Boolean).join(' · '))
  const aboutRows = $derived([...meta, ...details])
</script>

{#if open}
<!-- A named region, not a dialog. It was a dialog while it floated over the map,
     where a keyboard user who could not reach the close control was trapped
     behind it; the kit puts this card UNDER the map instead (.place-host), in
     the flow, covering nothing — so dialog semantics would now promise a modal
     that does not exist. Svelte said as much: a <section> cannot carry an
     interactive role. Escape still closes, because the reader whose focus is
     inside the card should not have to Tab back out to the close button. -->
<section
  class="sensor-panel"
  aria-labelledby="sensor-panel-title"
  tabindex="-1"
  onkeydown={(e) => { if (e.key === 'Escape') onclose() }}
>
  <!-- The close control sits on the title row, at its right edge: a bare word
       under the heading read as a link into somewhere rather than as the way
       out of this card. The glyph is inline SVG (the site ships no icon font),
       aria-hidden because the label beside it already names the action. -->
  <header>
    <div class="panel-heading">
      <h2 id="sensor-panel-title">{title}</h2>
      {#if subline}<p class="panel-sub">{subline}</p>{/if}
    </div>
    <div class="panel-actions">
      <!-- Moved (not copied) into the map dock or fullscreen sheet header while the card is there. -->
      {#if onstar}
        <button
          type="button"
          class="panel-star"
          aria-pressed={favourite}
          aria-label={favourite ? favRemove : favSave}
          title={favourite ? favRemove : favSave}
          onclick={onstar}
        ><span aria-hidden="true">{favourite ? '\u2605' : '\u2606'}</span></button>
      {/if}
      <button type="button" class="panel-info" aria-label={detailsLabel} onclick={() => { link.aboutOpen = true }}>
        <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" aria-hidden="true">
          <circle cx="8" cy="8" r="6.3" /><path d="M8 7.2v4M8 4.8v.1" stroke-linecap="round" />
        </svg>
      </button>
      <button type="button" class="panel-close" data-close onclick={onclose}>
        <svg class="panel-close__ico" width="16" height="16" viewBox="0 0 16 16"
             fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
          <path d="M3.5 3.5l9 9M12.5 3.5l-9 9" />
        </svg>
        <span class="panel-close__label">{closeLabel}</span>
      </button>
    </div>
  </header>

  {#if flagText}<p class="panel-flag">{flagText}</p>{/if}

  {#if meta.length}
    <dl class="panel-meta">
      {#each meta as row (row.key)}
        <dt>{row.label}</dt>
        <dd>{row.value}</dd>
      {/each}
    </dl>
  {/if}

  {#if network}<p class="panel-network">{network}</p>{/if}

  <div class="gauges">
    {#each rows as row (row.metric)}
      <Gauge
        label={row.label}
        value={row.missing ? noValue : number(row.value, locale)}
        unit={row.unit}
        band={row.band}
        model={row.model}
        missing={row.missing}
        pressed={link.metrics.includes(row.metric)}
        onselect={() => { link.metrics = [row.metric] }}
      />
    {/each}
  </div>

  {#if details.length}
    <!-- A native <details>, like the language picker: the same three states
         (closed, open, labelled) with no script and no aria bookkeeping. Closed
         is the default because this block describes the HARDWARE — devices,
         firmware, coordinates — and on a phone it stood between the readings
         and the chart, which are what a reader opens a station for. -->
    <details class="panel-details">
      <summary>{detailsLabel}</summary>
      <dl>
        {#each details as row (row.key)}
          <dt>{row.label}</dt>
          <dd>{row.value}</dd>
        {/each}
      </dl>
    </details>
  {/if}

  {#if chart}{@render chart()}{/if}

  {#if link.aboutOpen}
    <AboutSheet
      title={detailsLabel}
      {closeLabel}
      rows={aboutRows}
      {network}
      onclose={() => { link.aboutOpen = false }}
    />
  {/if}
</section>
{/if}
