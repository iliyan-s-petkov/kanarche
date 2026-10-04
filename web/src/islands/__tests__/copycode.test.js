// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { copySnippet, mount } from '../copycode.js'

function host(snippet) {
  const el = document.createElement('div')
  el.dataset.island = 'copycode'
  el.dataset.tCopy = 'Copy'
  el.dataset.tCopied = 'Copied'
  el.dataset.tFailed = 'Could not copy'
  const pre = document.createElement('pre')
  const code = document.createElement('code')
  code.textContent = snippet
  pre.append(code)
  el.append(pre)
  return el
}

function fakeTimers() {
  const q = []
  return {
    timer: (fn, ms) => { q.push({ fn, ms }); return q.length },
    clear: (id) => { if (q[id - 1]) q[id - 1].fn = () => {} },
    run: () => q.splice(0).forEach((t) => t.fn()),
    q,
  }
}

describe('copySnippet', () => {
  it('writes the exact text and reports success', async () => {
    const writeText = vi.fn().mockResolvedValue()
    expect(await copySnippet('a\n  b', { writeText })).toBe(true)
    expect(writeText).toHaveBeenCalledWith('a\n  b')
  })
  it('reports failure when the clipboard rejects or is missing', async () => {
    expect(await copySnippet('x', { writeText: () => Promise.reject(new Error('no')) })).toBe(false)
    expect(await copySnippet('x', undefined)).toBe(false)
  })
})

describe('mount', () => {
  it('adds a focusable Copy button and an aria-live status region', () => {
    const el = host('x')
    mount(el, { clipboard: { writeText: vi.fn() } })
    const b = el.querySelector('button')
    expect(b.type).toBe('button')
    expect(b.textContent).toBe('Copy')
    expect(b.tabIndex).toBe(0)
    expect(el.querySelector('[aria-live="polite"]')).not.toBeNull()
  })

  it('copies the code text untouched, shows Copied, then resets after RESET_MS', async () => {
    const snippet = '\n  <iframe src="x"\n        title="T"></iframe>  \n'
    const el = host(snippet)
    const writeText = vi.fn().mockResolvedValue()
    const t = fakeTimers()
    mount(el, { clipboard: { writeText }, timer: t.timer, clear: t.clear })
    const b = el.querySelector('button')
    b.click()
    await vi.waitFor(() => expect(b.textContent).toBe('Copied'))
    expect(writeText).toHaveBeenCalledWith(snippet)
    expect(el.querySelector('[aria-live]').textContent).toBe('Copied')
    expect(t.q.at(-1).ms).toBe(2000)
    t.run()
    expect(b.textContent).toBe('Copy')
    expect(el.querySelector('[aria-live]').textContent).toBe('')
  })

  it('shows the failure wording when the clipboard rejects', async () => {
    const el = host('x')
    const t = fakeTimers()
    mount(el, { clipboard: { writeText: () => Promise.reject(new Error('no')) }, timer: t.timer, clear: t.clear })
    const b = el.querySelector('button')
    b.click()
    await vi.waitFor(() => expect(b.textContent).toBe('Could not copy'))
    expect(el.querySelector('[aria-live]').textContent).toBe('Could not copy')
    t.run()
    expect(b.textContent).toBe('Copy')
  })

  it('restarts the reset timer on a second click', async () => {
    const el = host('x')
    const t = fakeTimers()
    const clear = vi.fn(t.clear)
    mount(el, { clipboard: { writeText: vi.fn().mockResolvedValue() }, timer: t.timer, clear })
    const b = el.querySelector('button')
    b.click()
    await vi.waitFor(() => expect(b.textContent).toBe('Copied'))
    b.click()
    await vi.waitFor(() => expect(t.q.length).toBe(2))
    // the first reset timer (id 1) is cancelled by the second click
    expect(clear).toHaveBeenLastCalledWith(1)
    t.run()
    expect(b.textContent).toBe('Copy')
  })
})
