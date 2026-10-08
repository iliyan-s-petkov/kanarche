import { test, expect } from './fixtures.js'

// OP #571: an oblast page opens below the sensor tier, and the count line under
// the filter read "0 of 0". sofia-oblast is the seed's oblast-kind area (e2e_test.go).
test('an oblast page counts its sensors below the sensor tier', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.goto('/en/area/sofia-oblast')

  // Faulty stations are hidden by default and not counted: 104's P2 is flagged
  // stuck, so four remain. 101-103 report; the EEA station has no P2.
  await expect(page.locator('.sensor-bar + .meta')).toHaveText('Showing 3 of 4 sensors, 1 with no recent readings')

  // The count came without the map drawing sensor dots at this tier: the marker
  // source holds area aggregates (slug), never sensors (id).
  const drawn = () => page.evaluate(async () => {
    const map = document.querySelector('[data-island="map"]').__map
    const feats = (await map.getSource('kanarche-data').getData()).features ?? []
    return { zoom: map.getZoom(), areas: feats.filter((f) => f.properties.slug).length, sensors: feats.filter((f) => f.properties.id != null).length }
  })
  await expect.poll(async () => (await drawn()).areas).toBeGreaterThan(0)
  const now = await drawn()
  expect(now.zoom).toBeLessThan(11)
  expect(now.sensors).toBe(0)
  await page.close()
})
