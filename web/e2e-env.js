// Reads a KANARCHE_E2E_* env var by suffix.
export function e2eEnv(suffix, env = process.env) {
  return env[`KANARCHE_E2E_${suffix}`]
}
