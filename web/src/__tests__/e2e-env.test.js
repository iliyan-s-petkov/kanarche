import { describe, it, expect } from 'vitest'
import { e2eEnv } from '../../e2e-env.js'

describe('e2eEnv', () => {
  it('reads KANARCHE_E2E_*', () => {
    expect(e2eEnv('BASE_URL', { KANARCHE_E2E_BASE_URL: 'new' })).toBe('new')
  })

  it('ignores the old AIRBG_E2E_* name', () => {
    expect(e2eEnv('BASE_URL', { AIRBG_E2E_BASE_URL: 'old' })).toBeUndefined()
  })

  it('returns undefined when the name is unset', () => {
    expect(e2eEnv('BASE_URL', {})).toBeUndefined()
  })
})
