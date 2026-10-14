<script>
  // One cell per season. The strip (card) runs oldest to newest over the last ten seasons, the grid (Seasons tab) newest first.
  import { seasonCells, stripCells, cellName } from '../lib/sea.js'
  import SeaSwatch from './SeaSwatch.svelte'

  let { classes, t, colours, variant = 'strip' } = $props()
  const shown = $derived(variant === 'grid' ? [...seasonCells(classes)].reverse() : stripCells(classes))
</script>

{#if shown.length}
  <ul class={`sea-seasons sea-seasons--${variant}`}>
    {#each shown as c (c.season)}
      <li>
        <!-- A focusable name-only image: the strip has nothing to press, but its cells must be reachable. -->
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <span class="sea-cell" role="img" tabindex="0" aria-label={cellName(c, t)}>
          <SeaSwatch quality={c.quality} {t} {colours} />
          {#if variant === 'grid'}<span class="sea-cell__year" aria-hidden="true">{c.season}</span>{/if}
        </span>
      </li>
    {/each}
  </ul>
  {#if variant === 'strip' && shown.length > 1}
    <p class="sea-seasons__ticks" aria-hidden="true"><span>{shown[0].season}</span><span>{shown[shown.length - 1].season}</span></p>
  {/if}
{/if}
