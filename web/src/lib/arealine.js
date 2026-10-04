// The panel's pointer to the area figures under the map: the same stats and area name the readout cards use.
import { areaStats, areaName } from './areastats.js'

// '' when the readouts would show no area row, so the panel has nothing to point at.
export function areaLine({ body, metric, sensorId, areas, slug, lang, t }) {
  const stats = areaStats(body, metric, sensorId)
  if (!stats) return ''
  const area = areaName(areas, slug, lang)
  const text = area ? t.areaBelow : t.areaBelowUnnamed
  return (text ?? '').replaceAll('{area}', area).replaceAll('{total}', String(stats.total))
}
