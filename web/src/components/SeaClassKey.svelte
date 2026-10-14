<script>
  // The five classes as swatch, initial and label.
  import { SEA_CLASSES, classLabel } from '../lib/sea.js'
  import SeaSwatch from './SeaSwatch.svelte'

  let { t, colours } = $props()
</script>

<ul class="sea-key" aria-label={t.legend}>
  {#each SEA_CLASSES as q (q)}
    <li class="sea-key__item"><SeaSwatch quality={q} {t} {colours} /><span>{classLabel(q, t)}</span></li>
  {/each}
</ul>
