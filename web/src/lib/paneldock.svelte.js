// Where the open card's parts are drawn. on: the map dock shows the title row, gauges and chart (the card under
// the map drops its gauges, star and info button). sheet: the fullscreen sheet shows the gauges and star.
let on = $state(false)
let sheet = $state(false)

export const panelDock = {
  get on() { return on },
  set on(next) { on = next },
  get sheet() { return sheet },
  set sheet(next) { sheet = next },
}
