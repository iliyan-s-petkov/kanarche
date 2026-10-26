<script>
  // The readout strip, in the shape base.gohtml's "readouts" define renders it.
  // Same classes and the same SVG presentation attributes — style-src is 'self'
  // with no 'unsafe-inline', so a style attribute would be dropped and the bar
  // would never paint.
  import { splitReadoutLabel } from '../lib/readoutLabel.js'
  import { spokenUnit } from '../lib/spokenunit.js'
  let { cards } = $props()
</script>

<div class="readouts">
  {#each cards as card (card.label + '|' + (card.group ?? ''))}
    {@const split = splitReadoutLabel(card.label, card.metric)}
    {@const spoken = spokenUnit(card.unit)}
    {@const speak = spoken !== card.unit}
    <div class="readout card">
      {#if card.group}<span class="readout__group">{card.group}</span>{/if}
      <span class="readout__label">{#if split.position === 'prefix'}<span class="readout__metric">{split.metric}{' · '}</span>{split.rest}{:else if split.position === 'suffix'}{split.rest}<span class="readout__metric">{' · '}{split.metric}</span>{:else}{split.rest}{/if}</span>
      <span class="readout__value">{card.value}{#if card.unit}{' '}<span class="readout__unit" aria-hidden={speak ? 'true' : undefined}>{card.unit}</span>{#if speak}<span class="sr-only"> {spoken}</span>{/if}{/if}</span>
      {#if card.band}<span class="readout__band">{card.band.word}</span>{/if}
      <span class="readout__tier">{card.tier}</span>
      {#if card.band}
        <svg class="readout__bar" viewBox="0 0 1 1" preserveAspectRatio="none" aria-hidden="true" focusable="false"><rect width="1" height="1" fill={card.band.colour}></rect></svg>
      {/if}
    </div>
  {/each}
</div>
