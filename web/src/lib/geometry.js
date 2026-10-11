// Mean of a polygon ring's vertices, dropping the closing vertex GeoJSON
// repeats to match the first: averaging it in would double-weight one corner.
export function ringCentroid(ring) {
  const [fx, fy] = ring[0]
  const last = ring[ring.length - 1]
  const pts = (last[0] === fx && last[1] === fy) ? ring.slice(0, -1) : ring
  const sum = pts.reduce(([sx, sy], [x, y]) => [sx + x, sy + y], [0, 0])
  return [sum[0] / pts.length, sum[1] / pts.length]
}

// The point a feature stands at: a Point's own coordinate, a Polygon's centroid.
export function featureCentre(geometry) {
  if (geometry?.type === 'Point') return geometry.coordinates
  if (geometry?.type === 'Polygon') return ringCentroid(geometry.coordinates[0])
  return null
}
