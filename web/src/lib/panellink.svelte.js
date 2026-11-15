// State the sensor card's parts share: the gauges (SensorPanel) and the chart
// controls (SensorChart) are mounted separately but pick the same metric,
// and the chart's menu opens the panel's "about" sheet. The map dock and the
// fullscreen sheet render their own gauges and chart from the same link, so
// the chart's window, range, nearby lines, reset and status live here too.
const STATUS_MS = 4000

export function createPanelLink() {
  let metrics = $state([])
  let aboutOpen = $state(false)
  // null until the first chart seeds it with the period the page opened on.
  let period = $state(null)
  let from = $state('')
  let to = $state('')
  let nearby = $state([])
  // Bumped by Reset only: remounting the chart discards the height the reader dragged it to.
  let resetToken = $state(0)
  let status = $state('')
  let statusTimer
  return {
    get metrics() { return metrics },
    set metrics(next) { metrics = next },
    get aboutOpen() { return aboutOpen },
    set aboutOpen(next) { aboutOpen = next },
    get period() { return period },
    set period(next) { period = next },
    get from() { return from },
    set from(next) { from = next },
    get to() { return to },
    set to(next) { to = next },
    get nearby() { return nearby },
    set nearby(next) { nearby = next },
    get resetToken() { return resetToken },
    set resetToken(next) { resetToken = next },
    get status() { return status },
    // Shows text in every status line of this card for a few seconds.
    say(text) {
      status = text
      clearTimeout(statusTimer)
      statusTimer = setTimeout(() => { status = '' }, STATUS_MS)
    },
  }
}
