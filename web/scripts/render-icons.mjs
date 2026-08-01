import { readFile } from 'node:fs/promises'
import { chromium } from '@playwright/test'

const svg = await readFile(new URL('../public/favicon.svg', import.meta.url), 'utf8')
const browser = await chromium.launch({ headless: true })
for (const size of [192, 512]) {
  const page = await browser.newPage({ viewport: { width: size, height: size }, deviceScaleFactor: 1 })
  await page.setContent(`<style>html,body{margin:0;width:${size}px;height:${size}px;overflow:hidden}svg{display:block;width:${size}px;height:${size}px}</style>${svg}`)
  const output = new URL(`../public/icon-${size}.png`, import.meta.url)
  await page.screenshot({ path: decodeURIComponent(output.pathname).replace(/^\/(.:\/)/, '$1'), omitBackground: true })
  await page.close()
}
await browser.close()

