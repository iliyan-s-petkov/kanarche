// lazy wraps a dynamic import so it runs once and every caller shares the promise.
// A rejected load is forgotten so a later call retries instead of caching the failure.
export function lazy(load) {
  let pending = null
  return () => {
    if (!pending) {
      pending = Promise.resolve().then(load).catch((err) => {
        pending = null
        throw err
      })
    }
    return pending
  }
}
