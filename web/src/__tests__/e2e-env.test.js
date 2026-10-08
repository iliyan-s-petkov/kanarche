import { describe, it, expect } from 'vitest'
import { e2eEnv } from '../../e2e-env.js'

describe('e2eEnv', () => {
  // The new KANARCHE_E2E_* name must win when both names are set.
  it('prefers KANARCHE_E2E_* over AIRBG_E2E_*', () => {
    const env = { KANARCHE_E2E_BASE_URL: 'new', AIRBG_E2E_BASE_URL: 'old' }
    expect(e2eEnv('BASE_URL', env)).toBe('new')
  })

  // A checkout still exporting the legacy name keeps working.
  it('falls back to AIRBG_E2E_* when the new name is unset', () => {
    expect(e2eEnv('BASE_URL', { AIRBG_E2E_BASE_URL: 'old' })).toBe('old')
  })

  it('returns undefined when neither name is set', () => {
    expect(e2eEnv('BASE_URL', {})).toBeUndefined()
  })
})
