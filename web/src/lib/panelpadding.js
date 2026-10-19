import { motion } from './mapmotion.js'
import { findSensor } from './sensors.svelte.js'

// Clear room kept between the open sensor's point and the panel's top edge.
const EDGE = 24

// Keeps the camera's bottom padding equal to what the bottom panel covers, and the open sensor above it.
// Padding moves the camera's centre up, so a sensor the panel would still cover is flown to the new centre.
export function installPanelPadding(map, dock, vs) {
  let covered = 0

  function keepSensorClear() {
    if (covered === 0) return
    const sensor = findSensor(vs.sensorId)
    if (!sensor || sensor.lon == null || sensor.lat == null) return
    const point = map.project([sensor.lon, sensor.lat])
    const height = map.getCanvas().clientHeight
    if (point.y > height - covered - EDGE || point.y < EDGE) {
      map.easeTo(motion({ center: [sensor.lon, sensor.lat], padding: { bottom: covered, top: 0, left: 0, right: 0 }, duration: 250 }))
    }
  }

  return dock.onLayout((height) => {
    if (height !== covered) {
      covered = height
      map.setPadding({ bottom: covered, top: 0, left: 0, right: 0 })
    }
    keepSensorClear()
  })
}
