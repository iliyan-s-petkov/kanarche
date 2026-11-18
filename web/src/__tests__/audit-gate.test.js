import { describe, expect, it } from 'vitest'
import { evaluateAudit } from '../../scripts/audit-gate.mjs'

const IGNORED = 'GHSA-fc5q-39xc-vjpf'
const OTHER = 'GHSA-aaaa-bbbb-cccc'
const ignore = { [IGNORED]: { reason: 'test', recheck: '2026-12-18' } }
const today = new Date('2026-11-18')

const advisory = (id, severity = 'high') => ({
  name: 'pkg',
  url: `https://github.com/advisories/${id}`,
  severity,
})
const report = (vulnerabilities) => ({ vulnerabilities })

describe('evaluateAudit', () => {
  it('passes when only the ignored advisory is reported', () => {
    const r = evaluateAudit(report({ picomatch: { severity: 'high', via: [advisory(IGNORED)] } }), ignore, today)
    expect(r.failures).toEqual([])
    expect(r.skipped).toEqual([IGNORED])
  })

  it('fails and names another high advisory', () => {
    const r = evaluateAudit(
      report({
        picomatch: { severity: 'high', via: [advisory(IGNORED)] },
        other: { severity: 'high', via: [advisory(OTHER)] },
      }),
      ignore,
      today,
    )
    expect(r.failures).toHaveLength(1)
    expect(r.failures[0]).toContain(OTHER)
  })

  it('fails on a critical advisory that is not ignored', () => {
    const r = evaluateAudit(report({ x: { severity: 'critical', via: [advisory(OTHER, 'critical')] } }), {}, today)
    expect(r.failures[0]).toContain(OTHER)
  })

  it('passes when only a moderate advisory is reported', () => {
    const r = evaluateAudit(report({ x: { severity: 'moderate', via: [advisory(OTHER, 'moderate')] } }), {}, today)
    expect(r.failures).toEqual([])
  })

  it('fails as stale when the ignored advisory is absent', () => {
    const r = evaluateAudit(report({}), ignore, today)
    expect(r.failures).toHaveLength(1)
    expect(r.failures[0]).toContain(IGNORED)
    expect(r.failures[0]).toContain('stale')
  })

  it('fails when the recheck date has passed', () => {
    const r = evaluateAudit(
      report({ picomatch: { severity: 'high', via: [advisory(IGNORED)] } }),
      ignore,
      new Date('2026-12-19'),
    )
    expect(r.failures).toHaveLength(1)
    expect(r.failures[0]).toContain('recheck')
  })

  it('passes on the recheck day itself', () => {
    const r = evaluateAudit(
      report({ picomatch: { severity: 'high', via: [advisory(IGNORED)] } }),
      ignore,
      new Date('2026-12-18'),
    )
    expect(r.failures).toEqual([])
  })

  it('resolves plain-string via chains to the root advisory', () => {
    const r = evaluateAudit(
      report({
        vite: { severity: 'high', via: ['picomatch', 'tinyglobby'] },
        tinyglobby: { severity: 'high', via: ['fdir', 'picomatch'] },
        fdir: { severity: 'high', via: ['picomatch'] },
        picomatch: { severity: 'high', via: [advisory(IGNORED)] },
      }),
      ignore,
      today,
    )
    expect(r.failures).toEqual([])
    expect(r.skipped).toEqual([IGNORED])
  })

  it('finds a non-ignored root advisory through a string chain', () => {
    const r = evaluateAudit(
      report({
        vite: { severity: 'high', via: ['dep'] },
        dep: { severity: 'high', via: [advisory(OTHER)] },
      }),
      ignore,
      today,
    )
    expect(r.failures.some((f) => f.includes(OTHER))).toBe(true)
  })

  it('survives a cyclic via chain', () => {
    const r = evaluateAudit(
      report({ a: { severity: 'high', via: ['b'] }, b: { severity: 'high', via: ['a'] } }),
      {},
      today,
    )
    expect(r.failures).toEqual([])
  })
})
