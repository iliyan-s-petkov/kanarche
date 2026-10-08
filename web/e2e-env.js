// Reads an E2E env var by suffix, preferring KANARCHE_E2E_* over the legacy AIRBG_E2E_*.
export function e2eEnv(suffix, env = process.env) {
  return env[`KANARCHE_E2E_${suffix}`] ?? env[`AIRBG_E2E_${suffix}`]
}
