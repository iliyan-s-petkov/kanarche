<script>
  // The ARIA 1.2 combobox: a text input that OWNS a listbox. Focus never leaves
  // the input — the active option is pointed at with aria-activedescendant — so
  // the reader hears each option without losing the caret they are still
  // typing into. That is also why the options are <li>, not buttons: a button
  // would take focus, and a control that steals the caret mid-word cannot be
  // typed into.
  //
  // The kit's own mockup stops at mouse picking. Its stylesheet and its
  // screen-reader hint both describe arrow keys and an active descendant, so
  // that contract is what is built here rather than the shorter thing the
  // mockup happens to do.
  import { matchAreas, exactMatch, splitMark } from '../lib/find.js'

  // `address` switches on the last-row address search (map tab only):
  // { search(q) -> Promise<rows>, onpick, onclear, row, loading, none, busy,
  //   error, credit, creditHref }. The request runs only on Enter or a click
  // on that row, never while typing.
  let { areas, lang = 'bg', label, placeholder, hint, empty, onpick, address = null, id = 'area-find' } = $props()

  let query = $state('')
  let open = $state(false)
  // The index of the keyboard cursor, not of a chosen thing: -1 means the
  // reader is still typing and Enter should fall back to an exact match.
  let active = $state(-1)

  // idle | loading | results | none | busy | error
  let addr = $state({ status: 'idle', rows: [] })
  // Bumped on every edit and every search so a late response is dropped.
  let seq = 0

  const MIN_ADDRESS_RUNES = 3
  const listId = `${id}-listbox`
  const matches = $derived(matchAreas(areas, query, lang))
  const trimmed = $derived(query.trim())
  const canSearch = $derived(!!address && [...trimmed].length >= MIN_ADDRESS_RUNES)
  const showResults = $derived(addr.status === 'results')
  const showStatus = $derived(addr.status !== 'idle' && addr.status !== 'results')
  // Cursor rows: the address results, or the area matches plus (always last)
  // the address row. A status line has no rows to move over.
  const count = $derived(
    showResults ? addr.rows.length : showStatus ? 0 : matches.length + (canSearch ? 1 : 0),
  )
  const activeId = $derived(active >= 0 && active < count ? `${id}-opt-${active}` : null)

  function resetAddress() {
    seq++
    if (addr.status !== 'idle') addr = { status: 'idle', rows: [] }
  }

  async function runSearch() {
    if (!canSearch) return
    const q = trimmed
    const mine = ++seq
    address.onclear?.()
    open = true
    active = -1
    addr = { status: 'loading', rows: [] }
    try {
      const rows = await address.search(q)
      if (mine !== seq) return
      addr = rows.length ? { status: 'results', rows } : { status: 'none', rows: [] }
    } catch (err) {
      if (mine !== seq) return
      addr = { status: err && err.kind === 'busy' ? 'busy' : 'error', rows: [] }
    }
  }

  function pickAddress(row) {
    query = row.label
    hide()
    address.onpick?.(row)
  }

  function oninput() {
    resetAddress()
    if (!query.trim()) address?.onclear?.()
    show()
  }

  function show() {
    open = true
    active = -1
  }
  function hide() {
    open = false
    active = -1
  }

  function pick(match) {
    if (!match) return
    query = match.name
    hide()
    onpick(match)
  }

  function move(step) {
    if (!count) return
    if (!open) open = true
    // Wraps, and an untouched cursor entering from the top lands on the first
    // option going down and on the last going up.
    const n = count
    active = active < 0 ? (step > 0 ? 0 : n - 1) : (active + step + n) % n
  }

  function onkeydown(e) {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        move(1)
        return
      case 'ArrowUp':
        e.preventDefault()
        move(-1)
        return
      case 'Home':
        if (!open) return
        e.preventDefault()
        active = 0
        return
      case 'End':
        if (!open) return
        e.preventDefault()
        active = count - 1
        return
      case 'Enter': {
        // The cursor if the reader moved it; otherwise only an unambiguous
        // query. A half-typed name never navigates.
        if (showResults) {
          if (active < 0) return
          e.preventDefault()
          pickAddress(addr.rows[active])
          return
        }
        const onAddressRow = canSearch && !showStatus && active === matches.length
        const target = active >= 0 ? matches[active] : exactMatch(matches, query, lang)
        if (onAddressRow || (!target && canSearch && !showStatus)) {
          e.preventDefault()
          runSearch()
          return
        }
        if (!target) return
        e.preventDefault()
        pick(target)
        return
      }
      case 'Escape':
        address?.onclear?.()
        // Two stages: the list goes first, the text second. One Escape that
        // did both would throw away a query the reader only wanted to see
        // past.
        if (open) {
          e.preventDefault()
          hide()
          return
        }
        if (query) {
          e.preventDefault()
          query = ''
          resetAddress()
        }
        return
      default:
    }
  }
</script>

<div class="field field--search combobox toolbar__find">
  <label class="field__label" for={id}>{label}</label>
  <input
    class="input"
    {id}
    type="search"
    autocomplete="off"
    role="combobox"
    aria-expanded={open}
    aria-controls={listId}
    aria-autocomplete="list"
    aria-activedescendant={activeId}
    aria-describedby="{id}-hint"
    {placeholder}
    bind:value={query}
    {oninput}
    onfocus={show}
    onkeydown={onkeydown}
    onblur={() => setTimeout(hide, 0)}
  >
  <!-- Announced, never painted: a sighted reader infers arrow keys from the
       open list, a screen-reader user does not, and a visible line of
       instructions under a search field is clutter for both. -->
  <span class="sr-only" id="{id}-hint">{hint}</span>
  <!-- tabindex -1: the list scrolls, so it would otherwise be a Tab stop that
       hides itself while focused. A press on it must not take focus either. -->
  <ul class="combobox__list" id={listId} role="listbox" tabindex="-1" hidden={!open} onmousedown={(e) => e.preventDefault()}>
    {#if showResults}
      {#each addr.rows as row, i (i)}
        <li
          class="combobox__opt"
          id="{id}-opt-{i}"
          role="option"
          aria-selected={i === active}
          onmousedown={(e) => { e.preventDefault(); pickAddress(row) }}
        ><span class="combobox__text">{row.label}</span></li>
      {/each}
      <li class="combobox__credit" role="presentation"><a href={address.creditHref} target="_blank" rel="noopener noreferrer">{address.credit}</a></li>
    {:else if showStatus}
      <li class="combobox__empty" data-state={addr.status}>{address[addr.status]}</li>
      {#if addr.status === 'none'}
        <li class="combobox__credit" role="presentation"><a href={address.creditHref} target="_blank" rel="noopener noreferrer">{address.credit}</a></li>
      {/if}
    {:else}
    {#if matches.length === 0}
      <!-- An absence stated plainly, not an error: typing a name this network
           has no area for is an ordinary thing to do. -->
      <li class="combobox__empty">{empty}</li>
    {:else}
      {#each matches as match, i (match.name)}
        {@const parts = splitMark(match.name, match.at, match.len)}
        <!-- mousedown, not click: click arrives after blur has already closed
             the list, so a mouse pick would land on nothing. -->
        <li
          class="combobox__opt"
          id="{id}-opt-{i}"
          role="option"
          aria-selected={i === active}
          onmousedown={(e) => { e.preventDefault(); pick(match) }}
        ><span class="combobox__text">{parts.before}<mark>{parts.hit}</mark>{parts.after}</span></li>
      {/each}
    {/if}
    {#if canSearch}
      <li
        class="combobox__opt combobox__opt--address"
        id="{id}-opt-{matches.length}"
        role="option"
        aria-selected={active === matches.length}
        onmousedown={(e) => { e.preventDefault(); runSearch() }}
      ><span class="combobox__text">{address.row} {trimmed}</span></li>
    {/if}
    {/if}
  </ul>
</div>
