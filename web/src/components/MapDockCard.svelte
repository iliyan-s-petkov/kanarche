<script>
  // The open sensor in the map dock (lib/sidedock.svelte.js): title row, gauges and the dock's chart, drawn from
  // the card model the panel island publishes (lib/sensorcard.js). The dock owns the grip, the history and area
  // buttons, its height and its mounting; this component owns everything that shows the sensor itself.
  import PanelGauges from './PanelGauges.svelte'
  import PanelStar from './PanelStar.svelte'
  import PanelInfo from './PanelInfo.svelte'

  // `ui.folded` is the dock's fold state; the fold button is named for what it does next.
  let { card, titleId, closeLabel = '', foldLabel = '', expandLabel = '', ui, onfold, onclose } = $props()
  const foldText = $derived(ui.folded ? expandLabel : foldLabel)
  // SensorChart comes with the card (card.chartComponent), so the map chrome does not import the chart library.
  const DockChart = $derived(card.chartComponent)
</script>

<div class="map-dock__head">
  <h2 id={titleId} class="map-dock__title">{card.title}</h2>
  {#if card.onstar}
    <PanelStar favourite={card.favourite} favSave={card.favSave} favRemove={card.favRemove} onstar={card.onstar} />
  {/if}
  <PanelInfo label={card.detailsLabel} link={card.link} />
  <button type="button" class="map-dock__fold" aria-label={foldText} title={foldText} onclick={onfold}>
    <svg class="map-dock__fold-ico" viewBox="0 0 16 16" aria-hidden="true">
      <path d="M3.5 6l4.5 4.5L12.5 6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
  </button>
  <button type="button" class="map-dock__close" aria-label={closeLabel} title={closeLabel} onclick={onclose}>
    <svg class="map-dock__close-ico" viewBox="0 0 16 16" aria-hidden="true">
      <path d="M3.5 3.5l9 9M12.5 3.5l-9 9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
  </button>
</div>

<div class="map-dock__body">
  <PanelGauges rows={card.rows} noValue={card.noValue} link={card.link} locale={card.locale} />
</div>

<!-- One props object per station (islands/panel.js), so a new station remounts the chart with its own state. -->
{#if card.chartProps && DockChart}
  {#key card.chartProps}
    <DockChart {...card.chartProps} place="dock" />
  {/key}
{/if}
