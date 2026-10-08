import { defineConfig, devices } from '@playwright/test'
import { e2eEnv } from './e2e-env.js'

// No webServer block: the Go test at internal/e2e/e2e_test.go owns the stack's
// lifetime, because internal/testsupport's Postgres helpers take a *testing.T
// and cannot be called from a plain command. The base URL arrives from it.
export default defineConfig({
  testDir: './e2e',
  // Serial, and no retries: a flaky E2E that passes on retry is a bug report
  // nobody reads. One worker also keeps the per-IP rate limiters — which see
  // every browser as the same client — out of the results.
  workers: 1,
  retries: 0,
  // Every spec cold-loads its own context, and a CI runner takes seconds, not
  // milliseconds, to fetch and evaluate the map chunk. The defaults (30s test,
  // 5s expect) were tuned for a laptop and made the slowest cold load the
  // failure. Local runs are unaffected: these are ceilings, not waits.
  timeout: 60_000,
  expect: { timeout: 15_000 },
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: e2eEnv('BASE_URL'),
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
