import { createApp, nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App.vue'

const responses: Record<string, unknown> = {
  '/api/v1/studios': { studios: [{ id: 1, name: '半岛2024', video_count: 2 }] },
  '/api/v1/videos': { videos: [{ id: 1, studio_id: 1, studio_name: '半岛2024', title: '灰姑娘与水晶鞋', duration_ms: 1554560, size_bytes: 467899649, bit_rate: 2407882, width: 1920, height: 1080, video_codec: 'h264', audio_codec: 'aac', compatibility: 'ready', published: true, available: true, unlocked: false, can_play: false, updated_at: '2026-07-31T00:00:00Z' }], page: 1, page_size: 20, total: 1 },
  '/api/v1/commerce': { video_price: 1, redeem_notice: '请在卡网兑换后输入卡密。' },
  '/api/v1/auth/captcha': { captcha_id: 'captcha-id', image: 'data:image/png;base64,iVBORw0KGgo=' },
}
const requests: string[] = []

describe('App', () => {
  beforeEach(() => {
    requests.length = 0
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost')
      requests.push(`${url.pathname}${url.search}`)
      if (url.pathname === '/api/v1/auth/me') return new Response(JSON.stringify({ error: { message: '请先登录' } }), { status: 401, headers: { 'Content-Type': 'application/json' } })
      return new Response(JSON.stringify(responses[url.pathname]), { status: 200, headers: { 'Content-Type': 'application/json' } })
    }))
  })
  afterEach(() => vi.unstubAllGlobals())

  it('renders the catalog as the first screen', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const app = createApp(App)
    app.mount(container)
    for (let index = 0; index < 5; index += 1) { await Promise.resolve(); await nextTick() }
    expect(container.textContent).toContain('小鹿放映室')
    expect(container.textContent).toContain('灰姑娘与水晶鞋')
    expect(container.textContent).toContain('推荐')
    expect(container.querySelector('.studio-strip')).not.toBeNull()
    expect(Array.from(container.querySelectorAll('.desktop-nav button')).some(button => button.textContent?.includes('工作室'))).toBe(false)
    const catalogRequest = requests.find(request => request.startsWith('/api/v1/videos?'))
    expect(catalogRequest).toMatch(/[?&]seed=\d+/)
    app.unmount()
    container.remove()
  })

  it('shows a captcha when switching to registration', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const app = createApp(App)
    app.mount(container)
    for (let index = 0; index < 5; index += 1) { await Promise.resolve(); await nextTick() }
    const login = Array.from(container.querySelectorAll('button')).find(button => button.textContent?.includes('登录')) as HTMLButtonElement
    login.click()
    await nextTick()
    const register = Array.from(container.querySelectorAll('button')).find(button => button.textContent?.includes('没有账号')) as HTMLButtonElement
    register.click()
    for (let index = 0; index < 5; index += 1) { await Promise.resolve(); await nextTick() }
    expect(container.querySelector('input[aria-label="验证码"]')).not.toBeNull()
    expect(requests).toContain('/api/v1/auth/captcha?purpose=register')
    app.unmount()
    container.remove()
  })
})
