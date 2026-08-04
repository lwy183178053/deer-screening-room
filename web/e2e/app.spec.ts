import { expect, test, type Page } from '@playwright/test'
import path from 'node:path'

const videos = Array.from({ length: 8 }, (_, index) => ({
  id: index + 1,
  studio_id: index < 4 ? 1 : 2,
  studio_name: index < 4 ? '半岛2024' : '鹿鸣工作室',
  title: index === 0 ? '新人试镜：一段需要在手机双列卡片中正确换行的较长作品标题' : index === 1 ? '灰姑娘与水晶鞋' : `工作室影像作品 ${index + 1}`,
  poster_url: `/api/v1/videos/${(index % 2) + 1}/poster`,
  duration_ms: 1_554_560 + index * 40_000,
  size_bytes: 467_899_649,
  bit_rate: 2_407_882,
  width: 1920,
  height: 1080,
  video_codec: 'h264',
  audio_codec: 'aac',
  compatibility: 'ready',
  published: true,
  available: true,
  unlocked: false,
  can_play: false,
  updated_at: '2026-07-31T00:00:00Z',
}))
const redeemNoticeURL = 'https://www.houfaka.com/liebiao/664EC2A2AD2B11C3A0F8D6B7E9C1234567890'
const redeemNotice = `鹿币购买地址\n${redeemNoticeURL}`

async function mockAPI(page: Page, admin = false) {
  const videoRequests: string[] = []
  let loginAttempts = 0
  const adminAccount = { id: 1, email: '3180615598@qq.com', is_admin: true, enabled: true, balance: 128, created_at: '2026-07-31T00:00:00Z' }
  const viewer = { id: 2, email: 'viewer@example.com', is_admin: false, enabled: true, balance: 20, created_at: '2026-07-31T00:00:00Z' }
  const redeemCodeRows = [
    { id: 1, credits: 10, used: false, created_at: '2026-07-31T00:00:00Z' },
    { id: 2, credits: 10, used: true, redeemed_by_email: viewer.email, redeemed_at: '2026-07-31T01:00:00Z', created_at: '2026-07-31T00:00:00Z' },
  ]
  await page.route('**/api/v1/**', async route => {
    const url = new URL(route.request().url())
    const pageNumber = Number(url.searchParams.get('page') ?? 1)
    const pageSize = 20
    if (/\/videos\/[12]\/poster$/.test(url.pathname)) {
      const poster = url.pathname.includes('/1/') ? 'poster-1.svg' : 'poster-2.svg'
      await route.fulfill({ path: path.resolve('e2e/assets', poster), contentType: 'image/svg+xml' })
      return
    }
    if (url.pathname === '/api/v1/playback/session/stream') {
      await route.fulfill({ status: 206, contentType: 'video/mp4', headers: { 'Content-Range': 'bytes 0-3/4' }, body: Buffer.from([0, 0, 0, 0]) })
      return
    }
    let body: unknown = {}
    let status = 200
    if (url.pathname === '/api/v1/studios') body = { studios: [{ id: 1, name: '半岛2024', video_count: 4 }, { id: 2, name: '鹿鸣工作室', video_count: 4 }] }
    else if (url.pathname === '/api/v1/auth/captcha') body = { captcha_id: 'captcha-id', image: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=' }
    else if (url.pathname === '/api/v1/auth/login') { loginAttempts += 1; status = 401; body = { error: { code: 'invalid_credentials', message: '邮箱或密码错误', captcha_required: loginAttempts > 0 } } }
    else if (url.pathname === '/api/v1/videos' || url.pathname === '/api/v1/library') {
      let catalog = [...videos]
      if (url.pathname === '/api/v1/videos') {
        videoRequests.push(`${url.pathname}${url.search}`)
        const studioID = Number(url.searchParams.get('studio_id') ?? 0)
        const query = (url.searchParams.get('q') ?? '').toLowerCase()
        const seed = Number(url.searchParams.get('seed') ?? 0)
        if (studioID) catalog = catalog.filter(video => video.studio_id === studioID)
        else if (query) catalog = catalog.filter(video => `${video.title} ${video.studio_name}`.toLowerCase().includes(query))
        else if (seed) catalog.sort((left, right) => ((((left.id * 2654435761) ^ seed) >>> 0) - (((right.id * 2654435761) ^ seed) >>> 0)) || left.id - right.id)
      }
      const pageVideos = catalog.slice((pageNumber - 1) * pageSize, pageNumber * pageSize)
      body = { videos: pageVideos, page: pageNumber, page_size: pageSize, total: catalog.length }
    }
    else if (/\/api\/v1\/videos\/\d+\/playback$/.test(url.pathname)) { status = 201; body = { id: 'session', stream_url: '/api/v1/playback/session/stream' } }
    else if (/\/api\/v1\/videos\/\d+$/.test(url.pathname)) {
      const video = videos[Number(url.pathname.split('/').at(-1)) - 1]
      body = { video: admin ? { ...video, can_play: true } : video }
    }
    else if (url.pathname === '/api/v1/commerce') body = { video_price: 1, redeem_notice: redeemNotice }
    else if (url.pathname === '/api/v1/auth/me' && admin) body = { account: adminAccount, csrf_token: 'csrf' }
    else if (url.pathname === '/api/v1/auth/me') { status = 401; body = { error: { message: '请先登录' } } }
    else if (url.pathname === '/api/v1/wallet') body = { balance: adminAccount.balance, entries: [] }
    else if (url.pathname === '/api/v1/admin/redeem-notice') body = { content: redeemNotice }
    else if (url.pathname === '/api/v1/admin/redeem-codes' && route.request().method() === 'GET') {
      const statusFilter = url.searchParams.get('status') ?? 'all'
      const query = (url.searchParams.get('q') ?? '').toLowerCase()
      let codes = redeemCodeRows.filter(code => statusFilter === 'all' || (statusFilter === 'used') === code.used)
      if (query) codes = codes.filter(code => code.redeemed_by_email?.toLowerCase().includes(query))
      body = { codes, page: 1, page_size: 20, total: codes.length, counts: { all: 2, unused: 1, used: 1 } }
    }
    else if (url.pathname === '/api/v1/admin/nodes' && route.request().method() === 'GET') body = { nodes: [{ id: 1, name: 'fnos-media', online: true, total_bytes: 1_000_000, available_bytes: 500_000, last_seen_at: '2026-07-31T00:00:00Z', scan_status: 'ok', last_scan_at: '2026-07-31T00:00:00Z' }] }
    else if (url.pathname === '/api/v1/admin/users' && route.request().method() === 'GET') body = { users: url.searchParams.get('q') ? [viewer] : [adminAccount, viewer], page: 1, page_size: 20, total: url.searchParams.get('q') ? 1 : 2, all_total: 2 }
    else if (/\/api\/v1\/admin\/users\/\d+\/credits$/.test(url.pathname)) {
      const input = route.request().postDataJSON() as { delta: number }
      viewer.balance += input.delta
      body = { account: viewer, adjusted: true }
    }
    await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
  })
  return { videoRequests }
}

test('desktop random catalog and vertical detail stay within the viewport', async ({ page }) => {
  const mocked = await mockAPI(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/')
  await expect(page.locator('.video-card')).toHaveCount(8)
  await expect(page.locator('.video-card')).toHaveCount(8)
  await expect(page.getByRole('button', { name: '工作室', exact: true })).toHaveCount(0)
  await expect(page.locator('.studio-strip')).toBeVisible()
  const firstSeed = new URL(mocked.videoRequests[0], 'http://localhost').searchParams.get('seed')
  await page.getByRole('button', { name: '首页', exact: true }).click()
  await expect.poll(() => mocked.videoRequests.length).toBeGreaterThan(1)
  const latestSeed = new URL(mocked.videoRequests.at(-1)!, 'http://localhost').searchParams.get('seed')
  expect(firstSeed).not.toBe(latestSeed)
  await page.locator('.video-card').first().click()
  await expect(page.getByRole('button', { name: '1 鹿币永久解锁' })).toBeVisible()
  await expect(page.getByText(/会员/)).toHaveCount(0)
  const vertical = await page.locator('.video-dialog').evaluate(element => {
    const poster = element.querySelector('.player-frame')!.getBoundingClientRect()
    const copy = element.querySelector('.video-dialog-copy')!.getBoundingClientRect()
    return poster.bottom <= copy.top
  })
  expect(vertical).toBe(true)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.screenshot({ path: 'test-results/catalog-desktop.png', fullPage: true })
})

test('mobile catalog keeps two columns and stable bottom navigation', async ({ page }) => {
  await mockAPI(page, true)
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  await expect(page.locator('.video-card')).toHaveCount(8)
  await expect(page.locator('.mobile-nav button')).toHaveCount(4)
  await page.waitForTimeout(800)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  const columns = await page.locator('.video-card').evaluateAll(cards => cards.slice(0, 2).map(card => Math.round(card.getBoundingClientRect().top)))
  expect(columns[0]).toBe(columns[1])
  await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight))
  const positions = await page.evaluate(() => ({ card: document.querySelector('.video-card:last-child')!.getBoundingClientRect().bottom, nav: document.querySelector('.mobile-nav')!.getBoundingClientRect().top }))
  expect(positions.card).toBeLessThanOrEqual(positions.nav)
  await page.screenshot({ path: 'test-results/catalog-mobile.png', fullPage: true })
})

test('member enters the dedicated watch route and can return', async ({ page }) => {
  await mockAPI(page, true)
  await page.goto('/')
  await page.locator('.video-card').first().click()
  await expect(page.getByRole('button', { name: '播放', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: '1 鹿币永久解锁' })).toBeVisible()
  await expect(page.getByText(/会员/)).toHaveCount(0)
  await page.getByRole('button', { name: '播放', exact: true }).click()
  await expect(page).toHaveURL(/\/watch\/\d+$/)
  await expect(page.locator('.watch-player .art-video-player')).toBeVisible()
  await expect(page.locator('.watch-main footer p')).toHaveCount(0)
  await expect(page.getByText(/1920×1080|H264|AAC/)).toHaveCount(0)
  await page.locator('.watch-player').hover()
  await expect(page.locator('.art-controls')).toBeVisible()
  await page.locator('.watch-player').click({ button: 'right' })
  await expect(page.locator('.watch-player a[href^="http"]')).toHaveCount(0)
  await expect(page.getByText(/ArtPlayer/i)).toHaveCount(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.screenshot({ path: 'test-results/watch-desktop.png', fullPage: true })
  await page.getByRole('button', { name: '返回' }).click()
  await expect(page).toHaveURL(/\/$/)
  await expect(page.locator('.video-grid')).toBeVisible()
  await expect(page.locator('.art-video-player')).toHaveCount(0)
})

test('administrator defaults to users and manages individual redeem codes', async ({ page }) => {
  await mockAPI(page, true)
  await page.setViewportSize({ width: 1280, height: 820 })
  await page.goto('/')
  await page.getByRole('button', { name: /管理/ }).first().click()
  await expect(page.getByRole('heading', { name: '用户账号' })).toBeVisible()
  await expect(page.getByText('共 2 个账号')).toBeVisible()
  await expect(page.locator('.admin-tabs button')).toHaveCount(3)
  await expect(page.getByRole('button', { name: '概览', exact: true })).toHaveCount(0)
  await page.getByLabel('搜索用户邮箱').fill('viewer')
  await page.getByRole('button', { name: '搜索', exact: true }).last().click()
  await expect(page.getByText('共 2 个账号')).toBeVisible()
  await expect(page.getByText('viewer@example.com')).toBeVisible()
  const viewerRow = page.locator('.table-row.user-admin').filter({ hasText: 'viewer@example.com' })
  await viewerRow.getByRole('button', { name: '增加鹿币' }).click()
  await page.getByLabel('数量').fill('12')
  await page.getByRole('button', { name: '确认调整' }).click()
  await expect(page.getByText('32', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '兑换码', exact: true }).click()
  await expect(page.getByRole('button', { name: '生成并导出 TXT', exact: true })).toBeVisible()
  await expect(page.getByText('批次名称')).toHaveCount(0)
  await expect(page.locator('.redeem-code-row:not(.head)')).toHaveCount(2)
  await page.getByRole('button', { name: '已使用 1', exact: true }).click()
  await expect(page.locator('.redeem-code-row:not(.head)')).toHaveCount(1)
  await expect(page.locator('.redeem-code-row:not(.head)')).toContainText('viewer@example.com')
  await page.getByLabel('搜索完整卡密或使用者邮箱').fill('viewer@example.com')
  await page.getByRole('button', { name: '搜索', exact: true }).last().click()
  await expect(page.locator('.redeem-code-row:not(.head)')).toHaveCount(1)
  await expect(page.locator('body')).not.toContainText('DEER-TEST-CODE')
  await expect(page.getByRole('heading', { name: '兑换获取说明' })).toBeVisible()
  await page.locator('textarea').fill('新的卡网兑换说明')
  await page.getByRole('button', { name: '保存说明' }).click()
  await expect(page.getByRole('button', { name: '设置', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '视频', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '订单', exact: true })).toHaveCount(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.screenshot({ path: 'test-results/admin-desktop.png', fullPage: true })
})

test('direct watch link restores playback after authentication state loads', async ({ page }) => {
  await mockAPI(page, true)
  await page.goto('/watch/1')
  await expect(page.locator('.watch-player .art-video-player')).toBeVisible()
  await expect(page.getByRole('heading', { name: videos[0].title })).toBeVisible()
})

test('studio filter stays on home and keeps the original catalog order', async ({ page }) => {
  const mocked = await mockAPI(page)
  await page.goto('/')
  await page.locator('.studio-strip button').filter({ hasText: '半岛2024' }).click()
  await expect(page.getByRole('heading', { name: '半岛2024' })).toBeVisible()
  await expect(page.locator('.video-card')).toHaveCount(4)
  const latestRequest = new URL(mocked.videoRequests.at(-1)!, 'http://localhost')
  expect(latestRequest.searchParams.get('studio_id')).toBe('1')
  expect(latestRequest.searchParams.has('seed')).toBe(false)
})

test('mobile watch page uses the branded player without overflow', async ({ page }) => {
  await mockAPI(page, true)
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/watch/1')
  await expect(page.locator('.watch-player .art-video-player')).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.screenshot({ path: 'test-results/watch-mobile.png', fullPage: true })
})

test('account page shows redemption and redeem instructions', async ({ page }) => {
  await mockAPI(page, true)
  await page.goto('/')
  await page.getByRole('button', { name: /128 鹿币/ }).click()
  await expect(page.getByRole('heading', { name: '充值鹿币' })).toBeVisible()
  await expect(page.getByText('鹿币购买地址')).toBeVisible()
  const purchaseLink = page.getByRole('link', { name: redeemNoticeURL })
  await expect(purchaseLink).toHaveAttribute('href', redeemNoticeURL)
  await expect(purchaseLink).toHaveAttribute('target', '_blank')
  await expect(purchaseLink).toHaveAttribute('rel', 'noopener noreferrer')
  await expect(page.getByText('易支付')).toHaveCount(0)
  await expect(page.getByText('支付宝')).toHaveCount(0)
  await page.screenshot({ path: 'test-results/account-desktop.png', fullPage: true })
})

for (const width of [320, 390, 430]) {
  test(`mobile account page wraps long purchase links at ${width}px`, async ({ page }) => {
    await mockAPI(page, true)
    await page.setViewportSize({ width, height: 844 })
    await page.goto('/')
    await page.getByRole('button', { name: '我的', exact: true }).click()
    await expect(page.locator('.account-email')).toBeVisible()
    await expect(page.getByRole('heading', { name: '充值鹿币' })).toBeVisible()
    await expect(page.locator('.redeem-form button')).toBeVisible()
    const purchaseLink = page.getByRole('link', { name: redeemNoticeURL })
    await expect(purchaseLink).toBeVisible()
    await expect(purchaseLink).toHaveCSS('overflow-wrap', 'anywhere')
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
    const linkBounds = await purchaseLink.boundingBox()
    expect(linkBounds).not.toBeNull()
    expect(linkBounds!.x).toBeGreaterThanOrEqual(0)
    expect(linkBounds!.x + linkBounds!.width).toBeLessThanOrEqual(width)
    expect(await page.locator('.account-email').evaluate(element => getComputedStyle(element).whiteSpace)).toBe('nowrap')
    await page.screenshot({ path: `test-results/account-mobile-${width}.png`, fullPage: true })
  })
}

test('failed login switches to the self-hosted captcha', async ({ page }) => {
  await mockAPI(page)
  await page.goto('/')
  await page.getByRole('button', { name: '登录', exact: true }).first().click()
  await page.getByLabel('邮箱').fill('viewer@example.com')
  await page.getByLabel('密码').fill('wrong-password')
  await page.getByRole('button', { name: '登录', exact: true }).last().click()
  await expect(page.getByRole('textbox', { name: '验证码', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: '刷新验证码' })).toBeVisible()
})

test('node page shows the latest in-memory scan status', async ({ page }) => {
  await mockAPI(page, true)
  await page.goto('/')
  await page.getByRole('button', { name: /管理/ }).first().click()
  await page.getByRole('button', { name: '节点', exact: true }).click()
  await expect(page.getByText('目录已同步')).toBeVisible()
  await expect(page.getByText('fnos-media')).toBeVisible()
})
