<script>
  import { MediaQuery } from 'svelte/reactivity'
  import MetricSwitcher from './MetricSwitcher.svelte'
  import MetricMenu from './MetricMenu.svelte'
  import { getSensorStatus, setSensorStatus } from '../lib/sensorfilter.svelte.js'
  import { getSensors } from '../lib/sensors.svelte.js'
  import { getShowFaulty } from '../lib/faultyfilter.svelte.js'
  import { countSensors, sensorCountLine, sensorShortLine } from '../lib/sensorcount.js'

  // `metric` arrives as a GETTER prop (see islands/sensorbar.js), the same shape
  // switcher.js uses: it reads through to the view state's $state, so the count
  // follows the reader's metric choice. A plain value would freeze at mount.
  let { texts, metric } = $props()

  // Phone width: the toolbar is one row, so the three pills collapse into a menu button.
  const narrow = typeof matchMedia === 'function' ? new MediaQuery('max-width: 672px') : { current: false }

  const status = $derived(getSensorStatus())
  // Silence is per metric: a sensor reporting humidity but not PM2.5 is silent
  // on the PM2.5 map, and that is what the map paints.
  const counts = $derived(countSensors(getSensors(), metric, { showFaulty: getShowFaulty() }))

  const options = $derived([
    { metric: 'all', label: texts.all },
    { metric: 'active', label: texts.active },
    { metric: 'inactive', label: texts.inactive },
  ])

  const line = $derived(sensorCountLine(texts, counts, status))
  const short = $derived(sensorShortLine(texts, counts, status))
</script>

<div class="sensor-bar">
  <!-- The kit also puts a place-name combobox in this bar. Not built: the
       sensors endpoint carries id, type, lon/lat, quality and metric columns —
       no place names — so the control would have nothing to search. A search
       box that finds nothing is worse than no search box. -->
  {#if narrow.current}
    <MetricMenu
      {options}
      selected={status}
      onselect={setSensorStatus}
      legend={texts.legend}
      name="sensor-status"
      id="sensor-status-menu"
      compact
    />
  {:else}
    <MetricSwitcher
      {options}
      selected={status}
      onselect={setSensorStatus}
      legend={texts.legend}
      name="sensor-status"
    />
  {/if}
</div>
<!-- Outside .sensor-bar, as the kit has it: the bar is a row of controls, and
     this is what the controls produced. Inside, it was a flex item and sat
     beside the radios rather than under them.

     role="status", not a hand-built live region: the numbers change as a RESULT
     of the reader's own click on the radios above, so they must be announced
     without taking focus off the control. -->
<p class="meta" role="status"><span class="meta__full">{line}</span><span class="meta__short" aria-hidden="true">{short}</span></p>
