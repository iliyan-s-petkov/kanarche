<script>
  // The favourite toggle on the card's title row, and on the dock's or sheet's while the card lends it.
  let { favourite = false, favSave = '', favRemove = '', onstar } = $props()
</script>

<button
  type="button"
  class="panel-star"
  aria-pressed={favourite}
  aria-label={favourite ? favRemove : favSave}
  title={favourite ? favRemove : favSave}
  onclick={onstar}
><span aria-hidden="true">{favourite ? '\u2605' : '\u2606'}</span></button>
