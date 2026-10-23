<script>
  // Draws the model from lib/gauge.js; no arithmetic here.
  import { arcPath, needlePoint } from '../lib/gauge.js'
  import { spokenUnit } from '../lib/spokenunit.js'

  let { label, value, unit, band = '', model = { fraction: null, colour: null, stops: [] }, missing = false, onselect = null, pressed = false } = $props()

  // The group variant has no content-derived name, so it keeps an author label.
  const ariaLabel = $derived(missing ? `${label}: ${value}` : `${label}: ${value} ${spokenUnit(unit)}${band ? `, ${band}` : ''}`)
  const spoken = $derived(spokenUnit(unit))
  const speak = $derived(spoken !== unit)
</script>

<svelte:element
  this={onselect ? 'button' : 'div'}
  class="gauge"
  class:gauge--pick={!!onselect}
  role={onselect ? undefined : 'group'}
  type={onselect ? 'button' : undefined}
  aria-pressed={onselect ? pressed : undefined}
  aria-label={onselect ? undefined : ariaLabel}
  onclick={onselect ?? undefined}
>
  <svg class="gauge__svg" viewBox="0 0 100 60" aria-hidden="true" focusable="false">
    {#if model.stops.length}
      <!-- Band track under the fill, at reduced opacity. -->
      {#each model.stops as stop, i (i)}
        <path
          class="gauge__band"
          d={arcPath(i === 0 ? 0 : model.stops[i - 1].fraction, stop.fraction)}
          stroke={stop.colour}
        />
      {/each}
    {:else}
      <path class="gauge__track-path" d={arcPath(0, 1)} />
    {/if}
    {#if model.fraction !== null}
      {@const tip = needlePoint(model.fraction)}
      {@const base = needlePoint(model.fraction, 22)}
      <line class="gauge__needle" x1={base.x} y1={base.y} x2={tip.x} y2={tip.y} />
      <circle class="gauge__pivot" cx="50" cy="50" r="3" />
    {/if}
  </svg>
  <span class="gauge__value">{#if missing}{value}{:else}{value}{' '}<span aria-hidden={speak ? 'true' : undefined}>{unit}</span>{#if speak}<span class="sr-only"> {spoken}</span>{/if}{/if}</span>
  <span class="gauge__label">{label}{#if band && !missing}<span class="gauge__name"> · {band}</span>{/if}</span>
</svelte:element>
