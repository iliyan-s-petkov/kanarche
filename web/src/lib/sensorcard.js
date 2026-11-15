// The open sensor card's model: the getters the panel island (islands/panel.js) hands SensorPanel, plus the
// chart's props. The map dock and the fullscreen sheet render from it instead of moving the card's nodes.
let card = null

export function setSensorCard(next) { card = next }
export function getSensorCard() { return card }
