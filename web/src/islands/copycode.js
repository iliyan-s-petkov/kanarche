// Copy button for the About page's embed snippet. Added by JS so a no-JS page has no dead control.
export const RESET_MS = 2000

// True when the text reached the clipboard; a missing or rejecting clipboard is a failure, not a throw.
export async function copySnippet(text, clipboard = globalThis.navigator?.clipboard) {
  try {
    await clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}

// clipboard and timers are parameters so a test can drive them without a real browser.
export function mount(el, { clipboard, timer = setTimeout, clear = clearTimeout } = {}) {
  const code = el.querySelector('code')
  if (!code) return
  const { tCopy = 'Copy', tCopied = 'Copied', tFailed = '' } = el.dataset

  const button = document.createElement('button')
  button.type = 'button'
  button.className = 'about-code__copy'
  button.textContent = tCopy

  const status = document.createElement('span')
  status.className = 'visually-hidden'
  status.setAttribute('role', 'status')
  status.setAttribute('aria-live', 'polite')

  let pending
  button.addEventListener('click', async () => {
    const ok = await copySnippet(code.textContent, clipboard)
    const msg = ok ? tCopied : tFailed || tCopy
    button.textContent = msg
    status.textContent = msg
    clear(pending)
    pending = timer(() => {
      button.textContent = tCopy
      status.textContent = ''
    }, RESET_MS)
  })

  el.classList.add('about-code-wrap--js')
  el.append(button, status)
}
