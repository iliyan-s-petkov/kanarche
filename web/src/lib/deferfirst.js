// Wraps a layer-menu apply so the first "on" waits for ready(); later calls run at once.
// While waiting, a toggle only changes what is applied when ready() resolves.
export function deferFirstOn(ready, run, warn = console.warn) {
  let wanted = true
  let waiting = false
  let started = false
  return (on) => {
    wanted = on
    if (waiting) return on
    const first = !started
    started = true
    if (!on || !first) return run(on)
    waiting = true
    ready()
      .then(() => {
        waiting = false
        return run(wanted)
      })
      .catch((err) => warn('deferred layer failed', err))
    return on
  }
}
