// Regenerates the README screenshots (docs/screenshots) from demo data, with
// no TrueNAS and no backend: Vite serves the UI, and the browser's /ws and
// /version are answered here.
//
//   npm run screenshots                 version from the latest git tag
//   NASWARDEN_VERSION=1.2.0 npm run screenshots
//
// Needs Chromium for Playwright once: npx playwright install --only-shell chromium

import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'
import { createServer } from 'vite'
import { demoState } from './demo-state.mjs'

const web = fileURLToPath(new URL('..', import.meta.url))
const out = fileURLToPath(new URL('../../docs/screenshots/', import.meta.url))

function version() {
  if (process.env.NASWARDEN_VERSION) return process.env.NASWARDEN_VERSION
  try {
    return execFileSync('git', ['describe', '--tags', '--abbrev=0'], { encoding: 'utf8' }).trim().replace(/^v/, '')
  } catch {
    return 'dev'
  }
}

const shots = [
  { name: 'overview', size: { width: 1440, height: 1000 }, act: async () => {} },
  { name: 'light', size: { width: 1440, height: 1000 }, scheme: 'light', act: async () => {} },
  {
    name: 'disks',
    size: { width: 1440, height: 1000 },
    act: async (page) => {
      await page.locator('.rail__item', { hasText: 'Disks' }).click()
      await page.locator('section[data-section=disks] .lt__row').first().click()
    },
  },
  {
    name: 'stacks',
    size: { width: 1440, height: 900 },
    act: async (page) => {
      await page.locator('.rail__item', { hasText: 'Stacks' }).click()
    },
  },
]

const server = await createServer({ root: web, logLevel: 'warn', server: { port: 0, strictPort: false } })
await server.listen()
const url = server.resolvedUrls.local[0]
const browser = await chromium.launch()
const build = { version: version(), commit: 'demo', date: new Date().toISOString() }

try {
  for (const shot of shots) {
    const page = await browser.newPage({ viewport: shot.size, colorScheme: shot.scheme ?? 'dark' })
    const timers = []
    const errors = []
    page.on('pageerror', (e) => errors.push(e.message))
    await page.route('**/version', (route) => route.fulfill({ json: build }))
    await page.routeWebSocket(/\/ws$/, (ws) => {
      const send = () => ws.send(JSON.stringify(demoState()))
      send()
      timers.push(setInterval(send, 2000))
    })
    await page.goto(url)
    await page.waitForSelector('.card--clickable')
    await page.waitForLoadState('networkidle') // app icons
    await shot.act(page)
    await page.waitForTimeout(800) // panel and view transitions
    await page.screenshot({ path: `${out}screenshot-${shot.name}.png` })
    timers.forEach(clearInterval)
    await page.close()
    if (errors.length) throw new Error(`${shot.name}: ${errors.join('; ')}`)
    console.log(`docs/screenshots/screenshot-${shot.name}.png`)
  }
} finally {
  await browser.close()
  await server.close()
}
