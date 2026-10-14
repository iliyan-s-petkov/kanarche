<script>
  // Samples of one season and one indicator on a log axis, with the zone's two limits as bands.
  // The geometry is lib/seachart.js. A chart is a picture, so it carries its text summary and
  // a table of the same readings behind a toggle.
  import { chartLayout, chartSummary } from '../lib/seachart.js'
  import { seaLevel, LEVEL_MARKS, formatDay, fillSlots, sampleSources, levelText } from '../lib/sea.js'

  let { points, limits, metricName, season, t, lang } = $props()

  let asTable = $state(false)
  const empty = $derived(points.length === 0)
  // chartLayout throws on no points, so it only runs when there are some.
  const layout = $derived(empty ? null : chartLayout(points, limits, (iso) => formatDay(iso, lang)))
  const summary = $derived(empty ? '' : chartSummary({ points, limits, template: t.chart.summary, metric: metricName, season }))
  const caption = $derived(fillSlots(t.chart.caption, { metric: metricName, season }))
  const bands = $derived([t.chart.bandWithin, t.chart.bandBetween, t.chart.bandAbove])
  const credits = $derived(sampleSources(points, t))
</script>

<figure class="sea-chart">
  {#if empty}
    <p class="sea-chart__empty">{t.noSamples}</p>
  {:else}
    {#if asTable}
      <table class="sea-chart__table">
        <caption>{caption}</caption>
        <thead><tr><th scope="col">{t.date}</th><th scope="col">{metricName} ({t.unit})</th></tr></thead>
        <tbody>
          {#each points as p, i (i)}
            {@const level = seaLevel(p.value, p.below, limits)}
            <tr>
              <th scope="row">{formatDay(p.date, lang)}</th>
              <td>
                {p.below ? '<' : ''}{p.value}
                {#if level}<span aria-hidden="true" class="sea-chart__mark"> {LEVEL_MARKS[level]}</span><span class="sr-only">, {levelText(t, level)}</span>{/if}
                {#if p.below}<span class="sr-only">, {t.belowDetection}</span>{/if}
                {#if p.preSeason}<span class="sea-chart__pre"> {t.preSeason}</span>{/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else}
      <svg class="sea-chart__svg" viewBox={`0 0 ${layout.width} ${layout.height}`} role="img" aria-label={summary}>
        {#each layout.bands as b (b.kind)}
          <rect class={`sea-chart__band sea-chart__band--${b.kind}`} x={layout.plot.x} y={b.y} width={layout.plot.w} height={b.h} />
        {/each}
        {#each layout.ticks as k (k.label)}
          <line class="sea-chart__grid" x1={layout.plot.x} x2={layout.plot.x + layout.plot.w} y1={k.y} y2={k.y} />
          <text class="sea-chart__tick" x={layout.plot.x - 4} y={k.y + 3} text-anchor="end">{k.label}</text>
        {/each}
        {#each layout.limitLines as l, i (i)}
          <line class="sea-chart__limit" x1={layout.plot.x} x2={layout.plot.x + layout.plot.w} y1={l.y} y2={l.y} />
          <text class="sea-chart__limit-label" x={layout.plot.x + layout.plot.w - 2} y={l.y - 2} text-anchor="end">{l.value}</text>
        {/each}
        {#each layout.dots as d, i (i)}
          <circle class={d.below ? 'sea-chart__dot sea-chart__dot--below' : 'sea-chart__dot'} cx={d.cx} cy={d.cy} r="4" />
        {/each}
        {#each layout.xLabels as x, i (i)}
          <text class="sea-chart__tick" x={x.x} y={layout.plot.y + layout.plot.h + 16} text-anchor={x.anchor}>{x.label}</text>
        {/each}
      </svg>
      <p class="sea-chart__unit">{t.unit}</p>
      <ul class="sea-chart__legend">
        {#each ['within', 'between', 'above'] as kind, i (kind)}
          <li><svg viewBox="0 0 12 12" class="sea-chart__swatch" aria-hidden="true" focusable="false"><rect class={`sea-chart__band sea-chart__band--${kind}`} width="12" height="12" /></svg>{bands[i]}</li>
        {/each}
        <li><svg viewBox="0 0 12 12" class="sea-chart__swatch" aria-hidden="true" focusable="false"><circle class="sea-chart__key-dot" cx="6" cy="6" r="4" /></svg>{t.chart.hollow}</li>
      </ul>
    {/if}
    {#each credits as c (c.id)}
      <p class="sea-chart__credit">{c.credit}</p>
    {/each}
    <button type="button" class="sea-chart__toggle" onclick={() => (asTable = !asTable)}>{asTable ? t.chart.showChart : t.chart.showTable}</button>
  {/if}
</figure>
