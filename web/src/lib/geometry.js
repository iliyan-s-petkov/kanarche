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

// Metres per degree of latitude, and the compass octant (0 north, clockwise) of a point
// from a centre. Equirectangular, fine at the few kilometres a map view spans.
const METRES_PER_DEGREE = 111195
export function offsetFrom(centre, point) {
  const dx = (point[0] - centre[0]) * Math.cos((centre[1] * Math.PI) / 180) * METRES_PER_DEGREE
  const dy = (point[1] - centre[1]) * METRES_PER_DEGREE
  const bearing = (Math.atan2(dx, dy) * 180) / Math.PI
  return { metres: Math.hypot(dx, dy), octant: ((Math.round(bearing / 45) % 8) + 8) % 8 }
}
