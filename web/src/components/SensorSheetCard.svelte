<script>
  // The open sensor in the fullscreen sheet (lib/sensorsheet.svelte.js): title, star and gauges, drawn from the
  // card model the panel island publishes (lib/sensorcard.js). The sheet owns its history button and mounting.
  import PanelGauges from './PanelGauges.svelte'
  import PanelStar from './PanelStar.svelte'

  let { card, titleId, closeLabel = '', onclose } = $props()
</script>

<div class="map-sensor-sheet__head">
  <h2 id={titleId} class="map-sensor-sheet__title">{card.title}</h2>
  {#if card.onstar}
    <PanelStar favourite={card.favourite} favSave={card.favSave} favRemove={card.favRemove} onstar={card.onstar} />
  {/if}
  <button type="button" class="map-sensor-sheet__close" aria-label={closeLabel} title={closeLabel} onclick={onclose}>
    <svg class="map-sensor-sheet__close-ico" viewBox="0 0 16 16" aria-hidden="true">
      <path d="M3.5 3.5l9 9M12.5 3.5l-9 9" fill="none" stroke="currentColor" stroke-width="1.5" />
    </svg>
  </button>
</div>

<div class="map-sensor-sheet__body">
  <PanelGauges rows={card.rows} noValue={card.noValue} link={card.link} locale={card.locale} />
</div>
