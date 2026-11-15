<script>
  // Opens the station's "about" sheet, which the card under the map renders (link.aboutOpen).
  let { label = '', link } = $props()
</script>

<button type="button" class="panel-info" aria-label={label} onclick={() => { link.aboutOpen = true }}>
  <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" aria-hidden="true">
    <circle cx="8" cy="8" r="6.3" /><path d="M8 7.2v4M8 4.8v.1" stroke-linecap="round" />
  </svg>
</button>
