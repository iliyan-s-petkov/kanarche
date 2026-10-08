// Runs fn when the main thread is idle; setTimeout where requestIdleCallback is missing.
export function afterIdle(fn, win = globalThis) {
  if (typeof win.requestIdleCallback === 'function') {
    win.requestIdleCallback(() => fn(), { timeout: 2000 })
    return
  }
  win.setTimeout(fn, 1)
}

// Resolves after the map's next 'idle' and an idle callback, or after maxMs if idle never comes.
export function whenRenderSettled(map, win = globalThis, maxMs = 5000) {
  return new Promise((resolve) => {
    let done = false
    let timer
    const finish = () => {
      if (done) return
      done = true
      win.clearTimeout?.(timer)
      afterIdle(resolve, win)
    }
    timer = win.setTimeout(finish, maxMs)
    map.once?.('idle', finish)
    if (!map.once) finish()
  })
}
