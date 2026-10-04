// Address search client. Plain fetch, no cache and no retry: the server caches,
// and a retry would spend the upstream's one request per second twice.
export class GeocodeError extends Error {
  constructor(kind) {
    super(kind)
    this.kind = kind
  }
}

// kind: 'busy' (server or per-visitor limit) or 'failed' (anything else).
export async function searchAddress(q, lang, fetchFn = (...a) => fetch(...a)) {
  const params = new URLSearchParams({ q, lang })
  let res
  try {
    res = await fetchFn(`/api/v1/geocode?${params}`, { headers: { Accept: 'application/json' } })
  } catch {
    throw new GeocodeError('failed')
  }
  if (res.status === 503 || res.status === 429) throw new GeocodeError('busy')
  if (!res.ok) throw new GeocodeError('failed')
  let body
  try {
    body = await res.json()
  } catch {
    throw new GeocodeError('failed')
  }
  if (!Array.isArray(body)) throw new GeocodeError('failed')
  return body
}
