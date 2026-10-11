<script>
  // Sensors in the current map view as real buttons, for keyboard and screen reader users.
  // Closed by default so it stays out of the way of the map and the readouts.
  let { items, total, rest, title, empty, more, onopen, onpreview, onunpreview } = $props()
</script>

<details class="nearby">
  <summary>{title} ({total})</summary>
  {#if items.length}
    <ul class="nearby__list">
      {#each items as item (item.id)}
        <li>
          <button
            type="button"
            class="nearby__item"
            onclick={() => onopen(item)}
            onfocus={() => onpreview(item)}
            onblur={onunpreview}
          >
            <span class="nearby__name">{item.name}</span>
            <span class="nearby__place">{item.place}</span>
            <span class="nearby__reading">{item.reading}</span>
          </button>
        </li>
      {/each}
    </ul>
    {#if rest > 0}<p class="nearby__more">{more.replace('{n}', rest)}</p>{/if}
  {:else}
    <p class="nearby__empty">{empty}</p>
  {/if}
</details>
