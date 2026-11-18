import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const GATED = new Set(['high', 'critical'])
const GHSA = /GHSA-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4}/i

// Collects the GHSA ids behind every high or critical vulnerability. String entries in via name another package, followed to its root advisory.
function collectAdvisories(vulnerabilities) {
  const found = new Set()
  const seen = new Set()
  const walk = (name) => {
    if (seen.has(name)) return
    seen.add(name)
    for (const v of vulnerabilities[name]?.via ?? []) {
      if (typeof v === 'string') {
        walk(v)
        continue
      }
      const id = GHSA.exec(v.url ?? '')?.[0]
      if (id && (v.severity === undefined || GATED.has(v.severity))) found.add(id)
    }
  }
  for (const [name, vuln] of Object.entries(vulnerabilities)) {
    if (GATED.has(vuln.severity)) walk(name)
  }
  return found
}

// Pure decision logic: failures is empty when the gate passes, skipped lists the ignored advisories still in effect.
export function evaluateAudit(audit, ignore, today = new Date()) {
  const reported = collectAdvisories(audit.vulnerabilities ?? {})
  const failures = []
  const skipped = []
  for (const id of reported) {
    if (!(id in ignore)) failures.push(`${id} is high or critical and not in audit-ignore.json`)
  }
  for (const [id, entry] of Object.entries(ignore)) {
    if (!reported.has(id)) {
      failures.push(`${id} is no longer reported, the exception is stale: remove it from audit-ignore.json`)
      continue
    }
    if (today > new Date(`${entry.recheck}T23:59:59Z`)) {
      failures.push(`${id} recheck date ${entry.recheck} has passed: look for a patched release and update audit-ignore.json`)
      continue
    }
    skipped.push(id)
  }
  return { failures, skipped }
}

function readAudit(file) {
  if (file) return JSON.parse(file === '-' ? readFileSync(0, 'utf8') : readFileSync(file, 'utf8'))
  try {
    return JSON.parse(execFileSync('npm', ['audit', '--json'], { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 }))
  } catch (err) {
    // npm audit exits non-zero when it finds anything, the JSON is still on stdout.
    if (err.stdout) return JSON.parse(err.stdout)
    throw err
  }
}

function main() {
  const ignorePath = fileURLToPath(new URL('../audit-ignore.json', import.meta.url))
  const ignore = JSON.parse(readFileSync(ignorePath, 'utf8'))
  const { failures, skipped } = evaluateAudit(readAudit(process.argv[2]), ignore)
  for (const id of skipped) console.log(`ignored ${id}: ${ignore[id].reason} (recheck ${ignore[id].recheck})`)
  for (const f of failures) console.error(`audit gate: ${f}`)
  process.exit(failures.length ? 1 : 0)
}

if (process.argv[1] === fileURLToPath(import.meta.url)) main()
