<script>
  // Map chrome: one icon. Tapping it opens a popover with "Refresh now" and
  // the auto-refresh interval, so there is a single refresh control on the page.
  let {
    status, minutes, options = [], labels, onpick, onrefresh, busy = false,
  } = $props()

  let open = $state(false)
  let root
  let trigger

  const id = 'data-refresh-panel'
  const hint = $derived(status ? `${labels.trigger} · ${status}` : labels.trigger)

  function close(refocus) {
    open = false
    if (refocus) trigger?.focus()
  }

  // Escape is handled here and stopped, so a window panel that hosts this on a
  // phone stays open and only the popover closes.
  function onkeydown(e) {
    if (e.key !== 'Escape' || !open) return
    e.stopPropagation()
    close(true)
  }

  function onoutside(e) {
    if (open && !root.contains(e.target)) close(false)
  }

  function refreshNow() {
    onrefresh()
    close(true)
  }
</script>

<svelte:document onmousedown={onoutside} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="data-refresh colmenu" bind:this={root} {onkeydown}>
  <!-- Clipped, not removed: a live region is the one reader who cannot hover. -->
  <span class="data-refresh__status sr-only" role="status">{status}</span>
  <!-- Not disabled while in flight: a disabled button drops keyboard focus. -->
  <button
    type="button"
    class="data-refresh__btn data-refresh__btn--icon"
    bind:this={trigger}
    aria-expanded={open}
    aria-controls={id}
    aria-busy={busy}
    title={hint}
    aria-label={hint}
    onclick={() => { open = !open }}
  >
    <svg class="data-refresh__ico" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true" focusable="false">
      <path d="M13.5 8a5.5 5.5 0 1 1-1.61-3.89" fill="none" stroke="currentColor" stroke-width="1.5"/>
      <path d="M13.5 2.5v3h-3" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="square"/>
    </svg>
  </button>
  <div class="colmenu__panel data-refresh__panel" {id} hidden={!open}>
    <button type="button" class="btn btn--secondary data-refresh__now" aria-busy={busy} onclick={refreshNow}>
      {labels.now}
    </button>
    <div role="radiogroup" aria-labelledby="{id}-label" class="data-refresh__group">
      <p class="data-refresh__legend" id="{id}-label">{labels.group}</p>
      {#each options as opt (opt.value)}
        <label class="colmenu__opt">
          <input
            type="radio"
            name="{id}-interval"
            value={opt.value}
            checked={opt.value === minutes}
            onchange={() => onpick(opt.value)}
          />
          <span>{opt.text}</span>
        </label>
      {/each}
    </div>
  </div>
</div>
