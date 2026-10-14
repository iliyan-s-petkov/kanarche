<script>
  // One class swatch: the class colour with its initial, so colour is never the only signal.
  import { seaClass, classInitial, inkFor } from '../lib/sea.js'

  let { quality, t, colours } = $props()
  const colour = $derived(colours[seaClass(quality)])
</script>

<svg class="sea-swatch" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
  <rect width="16" height="16" rx="3" fill={colour} />
  <text x="8" y="12" text-anchor="middle" class={`sea-swatch__ink sea-swatch__ink--${inkFor(colour)}`}>{classInitial(quality, t)}</text>
</svg>
