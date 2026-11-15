<script>
  // The open sensor's gauges, drawn by the card under the map and, while it lends them, by the map dock or the
  // fullscreen sheet. A tap picks the chart's metric through the shared link (lib/panellink.svelte.js).
  import Gauge from './Gauge.svelte'
  import { number } from '../lib/areacards.js'

  let { rows, noValue, link, locale = undefined } = $props()
</script>

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
