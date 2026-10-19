// The replay player's control and the speed steps it offers. The frame layer
// lives in ./timelapseframes.js so this file is all the map chunk carries of replay.

// Multipliers rather than millisecond values: FRAME_MS stays the one place the
// base rate is written down.
export const SPEEDS = [0.25, 0.5, 1, 2]
export const DEFAULT_SPEED = 0.5

export function speedLabel(speed) {
  return `${speed}\u00d7`
}

// The play control, unwired: the caller says what play, pause, scrub and exit
// do, because all four need the map. The scrubber stays hidden until there is
// something to scrub.
export function mountPlayer(frame, { label, playLabel, pauseLabel, exitLabel, speedLabel: speedName, host = frame }, doc = document) {
  const root = doc.createElement('div')
  root.className = 'map-play'

  const button = doc.createElement('button')
  button.type = 'button'
  button.className = 'btn map-play__btn'
  button.setAttribute('aria-label', playLabel)
  button.setAttribute('title', playLabel)
  button.setAttribute('aria-pressed', 'false')
  // Icon-only, like the refresh button beside it (DESIGN.md §5.2a).
  const glyph = doc.createElementNS('http://www.w3.org/2000/svg', 'svg')
  glyph.setAttribute('viewBox', '0 0 16 16')
  glyph.setAttribute('width', '14')
  glyph.setAttribute('height', '14')
  glyph.setAttribute('aria-hidden', 'true')
  glyph.setAttribute('focusable', 'false')
  const path = doc.createElementNS('http://www.w3.org/2000/svg', 'path')
  path.setAttribute('fill', 'currentColor')
  button.appendChild(glyph)
  glyph.appendChild(path)

  const slider = doc.createElement('input')
  slider.type = 'range'
  slider.className = 'map-play__scrub'
  slider.min = '0'
  slider.max = '0'
  slider.value = '0'
  slider.step = '1'
  slider.setAttribute('aria-label', label)
  slider.hidden = true

  const clock = doc.createElement('span')
  clock.className = 'map-play__clock'
  clock.hidden = true

  // What the clock cannot say: that the hour on screen is thinner than the rest,
  // or that there is no history to play. Live, because it appears mid-animation.
  const note = doc.createElement('span')
  note.className = 'map-play__note'
  note.setAttribute('aria-live', 'polite')
  note.hidden = true

  // Text, not an icon: "half speed" has no glyph a reader would recognise, and
  // the current speed has to be readable without opening the menu.
  const speedBox = doc.createElement('div')
  speedBox.className = 'colmenu map-play__speedmenu'
  speedBox.hidden = true
  const speed = doc.createElement('button')
  speed.type = 'button'
  speed.className = 'btn map-play__btn map-play__speed'
  speed.setAttribute('aria-label', speedName)
  speed.setAttribute('title', speedName)
  speed.setAttribute('aria-haspopup', 'menu')
  speed.setAttribute('aria-expanded', 'false')
  speed.textContent = speedLabel(DEFAULT_SPEED)
  speed.hidden = true
  const speedPanel = doc.createElement('div')
  speedPanel.className = 'colmenu__panel map-play__speedpanel'
  speedPanel.setAttribute('role', 'menu')
  speedPanel.setAttribute('aria-label', speedName)
  speedPanel.hidden = true
  const speedItems = SPEEDS.map((s) => {
    const item = doc.createElement('button')
    item.type = 'button'
    item.className = 'colmenu__opt map-play__speeditem'
    item.setAttribute('role', 'menuitemradio')
    item.setAttribute('aria-checked', 'false')
    item.tabIndex = -1
    item.textContent = speedLabel(s)
    item.dataset.speed = String(s)
    speedPanel.appendChild(item)
    return item
  })
  speedBox.appendChild(speed)
  speedBox.appendChild(speedPanel)

  // Its own button rather than a second meaning for the play button: pressing
  // play from a scrubbed frame replays, so without this there is no control that
  // returns the map to the live readings.
  const exit = doc.createElement('button')
  exit.type = 'button'
  exit.className = 'btn map-play__btn map-play__exit'
  exit.setAttribute('aria-label', exitLabel)
  exit.setAttribute('title', exitLabel)
  exit.hidden = true
  const exitGlyph = doc.createElementNS('http://www.w3.org/2000/svg', 'svg')
  exitGlyph.setAttribute('viewBox', '0 0 16 16')
  exitGlyph.setAttribute('width', '14')
  exitGlyph.setAttribute('height', '14')
  exitGlyph.setAttribute('aria-hidden', 'true')
  exitGlyph.setAttribute('focusable', 'false')
  const exitPath = doc.createElementNS('http://www.w3.org/2000/svg', 'path')
  exitPath.setAttribute('fill', 'currentColor')
  exitPath.setAttribute('d', 'M4.4 3.5 8 7.1l3.6-3.6 1 1L9 8.1l3.6 3.6-1 1L8 9.1l-3.6 3.6-1-1L7 8.1 3.4 4.5z')
  exit.appendChild(exitGlyph)
  exitGlyph.appendChild(exitPath)

  const PLAY = 'M5 3.5v9l7-4.5z'
  const PAUSE = 'M4.5 3.5h3v9h-3zM8.5 3.5h3v9h-3z'
  path.setAttribute('d', PLAY)

  const playing = (yes) => {
    path.setAttribute('d', yes ? PAUSE : PLAY)
    button.setAttribute('aria-pressed', String(yes))
    const name = yes ? pauseLabel : playLabel
    button.setAttribute('aria-label', name)
    button.setAttribute('title', name)
  }

  const toggles = []
  const scrubs = []
  const exits = []
  const speeds = []
  const menuOpen = (yes) => {
    speed.setAttribute('aria-expanded', String(yes))
    speedPanel.hidden = !yes
  }
  const focusItem = (i) => speedItems[(i + speedItems.length) % speedItems.length].focus()
  const checkedIndex = () => Math.max(0, speedItems.findIndex((it) => it.getAttribute('aria-checked') === 'true'))
  speed.addEventListener('click', () => {
    const yes = speedPanel.hidden
    menuOpen(yes)
    if (yes) focusItem(checkedIndex())
  })
  // Enter and Space already click a button; the arrows open it as well.
  speed.addEventListener('keydown', (e) => {
    if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
    e.preventDefault()
    menuOpen(true)
    focusItem(e.key === 'ArrowUp' ? speedItems.length - 1 : checkedIndex())
  })
  speedItems.forEach((item, i) => {
    item.addEventListener('click', () => {
      menuOpen(false)
      speed.focus()
      for (const fn of speeds) fn(SPEEDS[i])
    })
    item.addEventListener('keydown', (e) => {
      const step = { ArrowDown: 1, ArrowRight: 1, ArrowUp: -1, ArrowLeft: -1 }[e.key]
      if (step) {
        e.preventDefault()
        focusItem(i + step)
      } else if (e.key === 'Home' || e.key === 'End') {
        e.preventDefault()
        focusItem(e.key === 'Home' ? 0 : speedItems.length - 1)
      } else if (e.key === 'Tab') {
        menuOpen(false)
      }
    })
  })
  speedBox.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || speedPanel.hidden) return
    e.stopPropagation()
    menuOpen(false)
    speed.focus()
  })
  // pointerdown covers touch and mouse; a touch on the map closes the menu.
  doc.addEventListener('pointerdown', (e) => {
    if (!speedPanel.hidden && !speedBox.contains(e.target)) menuOpen(false)
  })
  // A press before the map has wired a listener is kept for the first one.
  let early = false
  button.addEventListener('click', () => {
    if (toggles.length === 0) early = true
    for (const fn of toggles) fn()
  })
  slider.addEventListener('input', () => {
    for (const fn of scrubs) fn(Number(slider.value))
  })
  exit.addEventListener('click', () => {
    // Collapse on the press: the island's exit handler is async, and the row
    // must not stay open across a network round trip.
    root.classList.remove('map-play--open')
    for (const fn of exits) fn()
  })

  root.appendChild(button)
  root.appendChild(slider)
  root.appendChild(clock)
  root.appendChild(note)
  root.appendChild(speedBox)
  root.appendChild(exit)
  host.appendChild(root)

  return {
    root, button, slider, clock, note, speed, speedPanel, speedItems, exit,
    playing,
    say: (text) => {
      note.textContent = text ?? ''
      note.hidden = !text
    },
    // All three together: any one of them on screen alone reads as a bug.
    // The class says the same thing to the sheet, which folds the whole row
    // behind the play button on a phone.
    show: (count) => {
      slider.max = String(Math.max(0, count - 1))
      slider.hidden = count <= 0
      clock.hidden = count <= 0
      speed.hidden = count <= 0
      speedBox.hidden = count <= 0
      if (count <= 0) menuOpen(false)
      exit.hidden = count <= 0
      root.classList.toggle('map-play--open', count > 0)
    },
    at: (i, text) => {
      slider.value = String(i)
      clock.textContent = text ?? ''
    },
    atSpeed: (s) => {
      speed.textContent = speedLabel(s)
      speed.setAttribute('aria-label', `${speedName}, ${speedLabel(s)}`)
      for (const item of speedItems) item.setAttribute('aria-checked', String(item.dataset.speed === String(s)))
    },
    ontoggle: (fn) => {
      toggles.push(fn)
      if (early) {
        early = false
        fn()
      }
    },
    onspeed: (fn) => speeds.push(fn),
    onscrub: (fn) => scrubs.push(fn),
    onexit: (fn) => exits.push(fn),
  }
}
