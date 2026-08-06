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
let authenticated = false
const walletPages: Record<number, unknown> = {}

describe('App', () => {
  beforeEach(() => {
    requests.length = 0
    authenticated = false
    for (const key of Object.keys(walletPages)) delete walletPages[Number(key)]
    vi.stubGlobal('scrollTo', vi.fn())
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost')
      requests.push(`${url.pathname}${url.search}`)
      if (url.pathname === '/api/v1/auth/me') {
        if (!authenticated) return new Response(JSON.stringify({ error: { message: '请先登录' } }), { status: 401, headers: { 'Content-Type': 'application/json' } })
        return new Response(JSON.stringify({ account: { id: 1, email: 'viewer@example.com', is_admin: false, enabled: true, balance: 3, created_at: '2026-07-31T00:00:00Z' }, csrf_token: 'csrf' }), { status: 200, headers: { 'Content-Type': 'application/json' } })
      }
      if (url.pathname === '/api/v1/wallet') return new Response(JSON.stringify(walletPages[Number(url.searchParams.get('page') || 1)] ?? { balance: 3, entries: [], page: 1, page_size: 20, total: 0 }), { status: 200, headers: { 'Content-Type': 'application/json' } })
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
    expect(container.querySelector('.studio-picker-trigger')).not.toBeNull()
    expect(container.querySelector('.studio-strip')).toBeNull()
    const picker = container.querySelector('.studio-picker-trigger') as HTMLButtonElement
    picker.click()
    await nextTick()
    expect(container.textContent).toContain('选择工作室')
    expect(container.querySelector('.studio-picker-rows .studio-picker-tag')).not.toBeNull()
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

  it('paginates wallet entries and replaces the current page', async () => {
    authenticated = true
    walletPages[1] = { balance: 3, entries: Array.from({ length: 20 }, (_, index) => ({ id: index + 1, delta: 1, kind: 'redeem', description: `第${index + 1}条`, created_at: '2026-07-31T00:00:00Z' })), page: 1, page_size: 20, total: 21 }
    walletPages[2] = { balance: 3, entries: [{ id: 21, delta: 1, kind: 'redeem', description: '第21条', created_at: '2026-07-31T00:00:00Z' }], page: 2, page_size: 20, total: 21 }
    const container = document.createElement('div')
    document.body.append(container)
    const app = createApp(App)
    app.mount(container)
    for (let index = 0; index < 6; index += 1) { await Promise.resolve(); await nextTick() }
    ;(container.querySelector('.account-chip') as HTMLButtonElement).click()
    for (let index = 0; index < 6; index += 1) { await Promise.resolve(); await nextTick() }
    expect(container.textContent).toContain('第1条')
    expect(container.textContent).not.toContain('第21条')
    const next = container.querySelector('[aria-label="下一页"]') as HTMLButtonElement
    expect(next).not.toBeNull()
    next.click()
    for (let index = 0; index < 6; index += 1) { await Promise.resolve(); await nextTick() }
    expect(container.textContent).toContain('第21条')
    expect(container.textContent).not.toContain('第1条')
    expect(requests).toContain('/api/v1/wallet?page=2')
    app.unmount()
    container.remove()
  })

})
