<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { AlertCircle, ArrowDown, ArrowLeft, ArrowRight, ArrowUp, CheckCircle2, Coins, Film, Home, KeyRound, Library, LogIn, LogOut, Minus, Play, Plus, RefreshCw, Search, Server, Shield, Ticket, User as UserIcon, Users } from '@lucide/vue'
import { api, APIError, csrf, setCSRF } from './api'
import { formatBytes, formatDate } from './format'
import { splitRedeemNoticeLinks } from './redeemNotice'
import type { Account, Commerce, NodeInfo, RedeemCode, RedeemCodeCounts, RedeemCodePage, Studio, UserPage, Video, VideoPage, WalletEntry } from './types'
import AppModal from './components/AppModal.vue'
import BrandLogo from './components/BrandLogo.vue'
import VideoCard from './components/VideoCard.vue'
import VideoPlayer from './components/VideoPlayer.vue'

type View = 'home' | 'library' | 'account' | 'admin'
type AdminTab = 'codes' | 'users' | 'nodes'

const account = ref<Account | null>(null)
const videos = ref<Video[]>([])
const studios = ref<Studio[]>([])
const commerce = ref<Commerce>({ video_price: 1, redeem_notice: '' })
const activeView = ref<View>('home')
const selectedStudio = ref(0)
const search = ref('')
const loading = ref(true)
const catalogPage = ref(1)
const catalogTotal = ref(0)
const catalogSeed = ref(createCatalogSeed())
const message = ref('')
const error = ref('')

const selectedVideo = ref<Video | null>(null)
const playerBusy = ref(false)
const watchMode = ref(false)
const watchVideo = ref<Video | null>(null)
const watchLoading = ref(false)
const watchError = ref('')
const streamURL = ref('')
const pendingWatchID = ref<number | null>(null)
let watchPushed = false
let scanPollTimer: number | undefined
let noticeTimer: number | undefined

const authOpen = ref(false)
const authMode = ref<'login' | 'register'>('login')
const authEmail = ref('')
const authPassword = ref('')
const authBusy = ref(false)
const authCaptchaRequired = ref(false)
const captchaID = ref('')
const captchaImage = ref('')
const captchaAnswer = ref('')

const walletEntries = ref<WalletEntry[]>([])
const redeemCode = ref('')

const adminTab = ref<AdminTab>('users')
const redeemCodes = ref<RedeemCode[]>([])
const redeemStatus = ref<'all' | 'used' | 'unused'>('all')
const redeemSearch = ref('')
const redeemPage = ref(1)
const redeemTotal = ref(0)
const redeemCounts = ref<RedeemCodeCounts>({ all: 0, used: 0, unused: 0 })
const redeemCodesLoading = ref(false)
const redeemNoticeDraft = ref('')
const redeemNoticeSaving = ref(false)
const users = ref<Account[]>([])
const userSearch = ref('')
const userPage = ref(1)
const userTotal = ref(0)
const userAllTotal = ref(0)
const usersLoadingMore = ref(false)
const nodes = ref<NodeInfo[]>([])
const nodeCreateOpen = ref(false)
const nodeName = ref('')
const nodeProvisionBusy = ref(false)
const nodeBundleTarget = ref<NodeInfo | null>(null)
const nodeBundleBusy = ref(false)
const nodeRotateBusy = ref(false)
const nodeDeleteTarget = ref<NodeInfo | null>(null)
const nodeDeleteBusy = ref(false)
const codeForm = ref({ credits: 1, count: 10 })
const resetUser = ref<Account | null>(null)
const resetPassword = ref('')
const creditTarget = ref<Account | null>(null)
const creditDirection = ref<1 | -1>(1)
const creditAmount = ref(1)
const creditReason = ref('')
const creditBusy = ref(false)
const redeemNoticeParts = computed(() => splitRedeemNoticeLinks(commerce.value.redeem_notice))
const redeemPageCount = computed(() => Math.max(1, Math.ceil(redeemTotal.value / 50)))

const sectionTitle = computed(() => {
  if (activeView.value === 'library') return '我的已购'
  if (activeView.value === 'account') return '鹿币与已购'
  if (activeView.value === 'admin') return '管理后台'
  if (selectedStudio.value) return studios.value.find(item => item.id === selectedStudio.value)?.name || '工作室'
  return search.value ? `搜索：${search.value}` : '推荐'
})
const creditDelta = computed(() => creditDirection.value * (Number(creditAmount.value) || 0))
const creditBalanceAfter = computed(() => (creditTarget.value?.balance ?? 0) + creditDelta.value)
const recommendationMode = computed(() => activeView.value === 'home' && !selectedStudio.value && !search.value.trim())
const catalogPageCount = computed(() => recommendationMode.value ? 1 : Math.max(1, Math.ceil(catalogTotal.value / 20)))

watch([message, error], () => {
  if (noticeTimer !== undefined) window.clearTimeout(noticeTimer)
  if (!message.value && !error.value) {
    noticeTimer = undefined
    return
  }
  noticeTimer = window.setTimeout(() => {
    message.value = ''
    error.value = ''
    noticeTimer = undefined
  }, 4000)
})

onMounted(async () => {
  window.addEventListener('popstate', syncLocation)
  await Promise.all([loadPublic(), loadMe()])
  loading.value = false
  await syncLocation()
})
onBeforeUnmount(() => {
  window.removeEventListener('popstate', syncLocation)
  stopWatch()
  if (scanPollTimer !== undefined) window.clearTimeout(scanPollTimer)
  if (noticeTimer !== undefined) window.clearTimeout(noticeTimer)
})

async function loadPublic() {
  try {
    const [studioBody, videoBody, commerceBody] = await Promise.all([
      api<{ studios: Studio[] }>('/api/v1/studios'),
      api<VideoPage>(`/api/v1/videos?seed=${catalogSeed.value}`),
      api<Commerce>('/api/v1/commerce'),
    ])
    studios.value = studioBody.studios ?? []
    videos.value = videoBody.videos ?? []
    catalogPage.value = videoBody.page || 1
    catalogTotal.value = recommendationMode.value ? Math.min(videoBody.total ?? videos.value.length, 20) : (videoBody.total ?? videos.value.length)
    commerce.value = commerceBody
  } catch (caught) { error.value = errorMessage(caught) }
}

async function loadMe() {
  try {
    const body = await api<{ account: Account; csrf_token: string }>('/api/v1/auth/me')
    account.value = body.account
    setCSRF(body.csrf_token)
  } catch { account.value = null; setCSRF('') }
}

async function refreshCatalog(reset = true) {
  const pageNumber = reset ? 1 : catalogPage.value
  const params = new URLSearchParams()
  const endpoint = activeView.value === 'library' ? '/api/v1/library' : '/api/v1/videos'
  if (endpoint === '/api/v1/videos' && search.value.trim()) params.set('q', search.value.trim())
  if (endpoint === '/api/v1/videos' && selectedStudio.value) params.set('studio_id', String(selectedStudio.value))
  if (endpoint === '/api/v1/videos' && !search.value.trim() && !selectedStudio.value) params.set('seed', String(catalogSeed.value))
  if (pageNumber > 1) params.set('page', String(pageNumber))
  try {
    const body = await api<VideoPage>(`${endpoint}${params.size ? `?${params}` : ''}`)
    const incoming = body.videos ?? []
    videos.value = incoming
    catalogPage.value = body.page || pageNumber
    catalogTotal.value = recommendationMode.value ? Math.min(body.total ?? videos.value.length, 20) : (body.total ?? videos.value.length)
  } catch (caught) {
    error.value = errorMessage(caught)
  }
}

async function goToCatalogPage(page: number) {
  if (page < 1 || page > catalogPageCount.value || page === catalogPage.value) return
  catalogPage.value = page
  await refreshCatalog(false)
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function openView(view: View) {
  if ((view === 'library' || view === 'account' || view === 'admin') && !account.value) { openAuth('login'); return }
  if (view === 'admin' && !account.value?.is_admin) return
  activeView.value = view
  error.value = ''
  if (view === 'home') { selectedStudio.value = 0; search.value = ''; catalogSeed.value = createCatalogSeed(); await refreshCatalog() }
  if (view === 'library') await refreshCatalog()
  if (view === 'account') await loadWallet()
  if (view === 'admin') await openAdmin('users')
}

async function chooseStudio(id: number) { selectedStudio.value = id; search.value = ''; activeView.value = 'home'; if (!id) catalogSeed.value = createCatalogSeed(); await refreshCatalog() }
async function submitSearch() { selectedStudio.value = 0; activeView.value = 'home'; await refreshCatalog() }

function openAuth(mode: 'login' | 'register') {
  authMode.value = mode; authOpen.value = true; error.value = ''; authCaptchaRequired.value = mode === 'register'; resetCaptcha()
  if (authCaptchaRequired.value) void loadCaptcha()
}
function resetCaptcha() { captchaID.value = ''; captchaImage.value = ''; captchaAnswer.value = '' }
async function loadCaptcha() {
  resetCaptcha()
  try {
    const body = await api<{ captcha_id: string; image: string }>(`/api/v1/auth/captcha?purpose=${authMode.value}`)
    captchaID.value = body.captcha_id; captchaImage.value = body.image
  } catch (caught) { error.value = errorMessage(caught) }
}
function switchAuthMode() { openAuth(authMode.value === 'login' ? 'register' : 'login') }
async function submitAuth() {
  authBusy.value = true; error.value = ''
  try {
    const body = await api<{ account: Account; csrf_token: string }>(`/api/v1/auth/${authMode.value}`, { method: 'POST', body: JSON.stringify({ email: authEmail.value, password: authPassword.value, captcha_id: captchaID.value, captcha_answer: captchaAnswer.value }) })
    account.value = body.account; setCSRF(body.csrf_token); authOpen.value = false; authPassword.value = ''; authCaptchaRequired.value = false; resetCaptcha()
    if (pendingWatchID.value) { const id = pendingWatchID.value; pendingWatchID.value = null; await openWatch(id, false) } else { await refreshCatalog() }
  } catch (caught) {
    error.value = errorMessage(caught)
    if (authMode.value === 'register' || (caught instanceof APIError && caught.captchaRequired)) {
      authCaptchaRequired.value = true
      await loadCaptcha()
    }
  } finally { authBusy.value = false }
}
async function logout() { await api('/api/v1/auth/logout', { method: 'POST' }); account.value = null; setCSRF(''); activeView.value = 'home'; selectedStudio.value = 0; search.value = ''; catalogSeed.value = createCatalogSeed(); await refreshCatalog() }

async function openVideo(video: Video) {
  try { const body = await api<{ video: Video }>(`/api/v1/videos/${video.id}`); selectedVideo.value = body.video } catch { selectedVideo.value = video }
}
async function startPlayback() {
  if (!account.value) { selectedVideo.value = null; openAuth('login'); return }
  if (!selectedVideo.value) return
  playerBusy.value = true
  try { await openWatch(selectedVideo.value.id, true) } finally { playerBusy.value = false }
}
async function unlockVideo() {
  if (!account.value) { selectedVideo.value = null; openAuth('login'); return }
  if (!selectedVideo.value) return
  try { await api(`/api/v1/videos/${selectedVideo.value.id}/unlock`, { method: 'POST', body: '{}' }); await Promise.all([loadMe(), refreshCatalog()]); const body = await api<{ video: Video }>(`/api/v1/videos/${selectedVideo.value.id}`); selectedVideo.value = body.video; message.value = '已永久解锁本片。' } catch (caught) { error.value = errorMessage(caught) }
}
function watchIDFromLocation() {
  const match = location.pathname.match(/^\/watch\/(\d+)\/?$/)
  return match ? Number(match[1]) : null
}
async function syncLocation() {
  const id = watchIDFromLocation()
  if (id) { await openWatch(id, false); return }
  stopWatch()
}
async function openWatch(videoID: number, push: boolean) {
  if (push) {
    history.pushState({}, '', `/watch/${videoID}`)
    watchPushed = true
  }
  watchMode.value = true
  watchLoading.value = true
  watchError.value = ''
  streamURL.value = ''
  selectedVideo.value = null
  if (!account.value) {
    pendingWatchID.value = videoID
    watchLoading.value = false
    watchError.value = '登录后继续播放'
    openAuth('login')
    return
  }
  try {
    const detail = await api<{ video: Video }>(`/api/v1/videos/${videoID}`)
    watchVideo.value = detail.video
    if (!detail.video.available) throw new Error('媒体节点暂时不可用')
    if (!detail.video.can_play) throw new Error('尚未获得本片观看权限')
    const playback = await api<{ stream_url: string }>(`/api/v1/videos/${videoID}/playback`, { method: 'POST', body: '{}' })
    streamURL.value = playback.stream_url
  } catch (caught) {
    watchError.value = errorMessage(caught)
  } finally {
    watchLoading.value = false
  }
}
function stopWatch() {
  streamURL.value = ''
  watchVideo.value = null
  watchMode.value = false
  watchLoading.value = false
  watchError.value = ''
  pendingWatchID.value = null
}
function backFromWatch() {
  if (watchPushed) {
    watchPushed = false
    history.back()
    return
  }
  history.replaceState({}, '', '/')
  stopWatch()
}

async function loadWallet() { const body = await api<{ entries: WalletEntry[] }>('/api/v1/wallet'); walletEntries.value = body.entries ?? []; await loadMe() }
async function redeem() { try { await api('/api/v1/wallet/redeem', { method: 'POST', body: JSON.stringify({ code: redeemCode.value }) }); redeemCode.value = ''; await loadWallet(); message.value = '兑换成功，鹿币已到账。' } catch (caught) { error.value = errorMessage(caught) } }

async function openAdmin(tab: AdminTab) {
  adminTab.value = tab
  if (tab === 'codes') {
    const [, noticeBody] = await Promise.all([
      loadRedeemCodes(),
      api<{ content: string }>('/api/v1/admin/redeem-notice'),
    ])
    redeemNoticeDraft.value = noticeBody.content ?? ''
  }
  if (tab === 'users') await loadUsers()
  if (tab === 'nodes') { const body = await api<{ nodes: NodeInfo[] }>('/api/v1/admin/nodes'); nodes.value = body.nodes ?? [] }
}
async function loadUsers(reset = true) {
  const pageNumber = reset ? 1 : userPage.value + 1
  const params = new URLSearchParams()
  if (userSearch.value.trim()) params.set('q', userSearch.value.trim())
  if (pageNumber > 1) params.set('page', String(pageNumber))
  if (!reset) usersLoadingMore.value = true
  try {
    const body = await api<UserPage>(`/api/v1/admin/users${params.size ? `?${params}` : ''}`)
    users.value = reset ? body.users ?? [] : [...users.value, ...(body.users ?? [])]
    userPage.value = body.page || pageNumber
    userTotal.value = body.total ?? users.value.length
    userAllTotal.value = body.all_total ?? userAllTotal.value
  } finally { usersLoadingMore.value = false }
}
async function submitUserSearch() { await loadUsers() }
async function loadMoreUsers() { if (!usersLoadingMore.value && users.value.length < userTotal.value) await loadUsers(false) }
async function loadRedeemCodes(reset = true) {
  const pageNumber = reset ? 1 : redeemPage.value
  const params = new URLSearchParams({ status: redeemStatus.value })
  if (redeemSearch.value.trim()) params.set('q', redeemSearch.value.trim())
  if (pageNumber > 1) params.set('page', String(pageNumber))
  redeemCodesLoading.value = true
  try {
    const body = await api<RedeemCodePage>(`/api/v1/admin/redeem-codes?${params}`)
    const incoming = body.codes ?? []
    redeemCodes.value = incoming
    redeemPage.value = body.page || pageNumber
    redeemTotal.value = body.total ?? redeemCodes.value.length
    redeemCounts.value = body.counts ?? redeemCounts.value
  } finally { redeemCodesLoading.value = false }
}
async function selectRedeemStatus(status: 'all' | 'used' | 'unused') { if (redeemStatus.value !== status) { redeemStatus.value = status; await loadRedeemCodes() } }
async function submitRedeemSearch() { await loadRedeemCodes() }
async function goToRedeemPage(page: number) {
  if (page < 1 || page > redeemPageCount.value || page === redeemPage.value || redeemCodesLoading.value) return
  redeemPage.value = page
  await loadRedeemCodes(false)
  window.scrollTo({ top: 0, behavior: 'smooth' })
}
async function createCodes() {
  error.value = ''
  const response = await fetch('/api/v1/admin/redeem-codes', { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf() }, body: JSON.stringify(codeForm.value) })
  if (!response.ok) { const body = await response.json().catch(() => null); error.value = body?.error?.message || '生成兑换码失败'; return }
  const blob = await response.blob(); const link = document.createElement('a'); link.href = URL.createObjectURL(blob); link.download = `deer-codes-${Date.now()}.txt`; link.click(); URL.revokeObjectURL(link.href); await loadRedeemCodes()
}
async function saveRedeemNotice() {
  redeemNoticeSaving.value = true
  try {
    const body = await api<{ content: string }>('/api/v1/admin/redeem-notice', { method: 'PUT', body: JSON.stringify({ content: redeemNoticeDraft.value }) })
    redeemNoticeDraft.value = body.content
    commerce.value.redeem_notice = body.content
    message.value = '兑换说明已保存。'
  } catch (caught) { error.value = errorMessage(caught) } finally { redeemNoticeSaving.value = false }
}
async function toggleUser(user: Account) { await api(`/api/v1/admin/users/${user.id}`, { method: 'PATCH', body: JSON.stringify({ enabled: !user.enabled }) }); await loadUsers() }
function openCreditAdjustment(user: Account, direction: 1 | -1) { creditTarget.value = user; creditDirection.value = direction; creditAmount.value = 1; creditReason.value = '' }
async function submitCreditAdjustment() {
  if (!creditTarget.value || creditAmount.value < 1 || creditAmount.value > 100000) return
  creditBusy.value = true
  try {
    const body = await api<{ account: Account; adjusted: boolean }>(`/api/v1/admin/users/${creditTarget.value.id}/credits`, { method: 'POST', body: JSON.stringify({ delta: creditDelta.value, reason: creditReason.value, request_id: crypto.randomUUID() }) })
    const user = users.value.find(item => item.id === body.account.id); if (user) Object.assign(user, body.account)
    if (account.value?.id === body.account.id) Object.assign(account.value, body.account)
    creditTarget.value = null
    message.value = creditDirection.value > 0 ? '鹿币已增加。' : '鹿币已扣减。'
  } catch (caught) { error.value = errorMessage(caught) } finally { creditBusy.value = false }
}
async function submitPasswordReset() { if (!resetUser.value) return; try { await api(`/api/v1/admin/users/${resetUser.value.id}/reset-password`, { method: 'POST', body: JSON.stringify({ password: resetPassword.value }) }); resetUser.value = null; resetPassword.value = ''; message.value = '密码已重置，用户现有会话已退出。' } catch (caught) { error.value = errorMessage(caught) } }
async function rescan(node: NodeInfo) {
  await api(`/api/v1/admin/nodes/${node.id}/rescan`, { method: 'POST', body: '{}' })
  const previousScanAt = node.last_scan_at || ''
  node.scan_status = 'scanning'
  node.scan_error = ''
  message.value = '已要求节点重新扫描，完成后会自动刷新目录。'
  if (scanPollTimer !== undefined) window.clearTimeout(scanPollTimer)
  void pollScan(node.id, previousScanAt, Date.now() + 15 * 60 * 1000)
}

function openNodeCreate() {
  nodeName.value = ''
  nodeCreateOpen.value = true
  error.value = ''
}

async function provisionNode() {
  nodeProvisionBusy.value = true
  error.value = ''
  try {
    const body = await api<{ node: NodeInfo; bundle_url: string }>('/api/v1/admin/nodes/provision', { method: 'POST', body: JSON.stringify({ name: nodeName.value.trim() }) })
    nodes.value = [...nodes.value, body.node]
    nodeBundleTarget.value = body.node
    nodeCreateOpen.value = false
    message.value = '节点已创建，请下载一次性安装包。'
  } catch (caught) { error.value = errorMessage(caught) } finally { nodeProvisionBusy.value = false }
}

async function downloadNodeBundle(node: NodeInfo) {
  nodeBundleBusy.value = true
  error.value = ''
  try {
    const response = await fetch(`/api/v1/admin/nodes/${node.id}/bundle`, { credentials: 'same-origin' })
    if (!response.ok) {
      const body = await response.json().catch(() => null)
      throw new Error(body?.error?.message || '安装包下载失败')
    }
    const blob = await response.blob()
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = `deer-${node.name}.zip`
    link.click()
    URL.revokeObjectURL(link.href)
    node.bundle_downloaded = true
    nodeBundleTarget.value = null
    message.value = '节点安装包已下载。'
  } catch (caught) { error.value = errorMessage(caught) } finally { nodeBundleBusy.value = false }
}

async function rotateNode(node: NodeInfo) {
  nodeRotateBusy.value = true
  error.value = ''
  try {
    await api(`/api/v1/admin/nodes/${node.id}/rotate`, { method: 'POST', body: '{}' })
    node.bundle_downloaded = false
    nodeBundleTarget.value = node
    message.value = '凭据已轮换，请重新下载安装包。'
  } catch (caught) { error.value = errorMessage(caught) } finally { nodeRotateBusy.value = false }
}

async function deleteNode() {
  if (!nodeDeleteTarget.value) return
  nodeDeleteBusy.value = true
  error.value = ''
  try {
    await api(`/api/v1/admin/nodes/${nodeDeleteTarget.value.id}`, { method: 'DELETE' })
    nodes.value = nodes.value.filter(item => item.id !== nodeDeleteTarget.value?.id)
    nodeBundleTarget.value = null
    nodeDeleteTarget.value = null
    message.value = '节点及其目录权益已删除，源视频文件保留。'
    await loadPublic()
  } catch (caught) { error.value = errorMessage(caught) } finally { nodeDeleteBusy.value = false }
}

async function pollScan(nodeID: number, previousScanAt: string, deadline: number) {
  const wait = async () => new Promise<void>(resolve => { scanPollTimer = window.setTimeout(resolve, 2000) })
  while (Date.now() < deadline) {
    await wait()
    try {
      const body = await api<{ nodes: NodeInfo[] }>('/api/v1/admin/nodes')
      const current = body.nodes?.find(item => item.id === nodeID)
      if (!current) return
      const finished = current.scan_status === 'error' || (current.scan_status !== 'scanning' && current.last_scan_at !== previousScanAt)
      if (!finished) continue
      const target = nodes.value.find(item => item.id === nodeID)
      if (target) Object.assign(target, current)
      if (current.scan_status === 'error') {
        message.value = current.scan_error ? `节点扫描失败：${current.scan_error}` : '节点扫描失败，请检查节点日志。'
        return
      }
      await loadPublic()
      message.value = '节点扫描完成，视频目录已刷新。'
      return
    } catch {
      // Keep polling while the node reports an in-progress scan.
    }
  }
  message.value = '节点扫描时间较长，请稍后刷新节点状态。'
}

function errorMessage(value: unknown) { return value instanceof Error ? value.message : '操作失败，请稍后重试' }
function createCatalogSeed() { return crypto.getRandomValues(new Uint32Array(1))[0] || 1 }
function scanStatus(node: NodeInfo) { if (node.scan_status === 'scanning') return '扫描中'; if (node.scan_status === 'error') return '扫描失败'; if (node.scan_status === 'ok') return '目录已同步'; return '等待扫描状态' }
</script>

<template>
  <div class="app-shell" @dragstart.prevent @dragover.prevent @drop.prevent>
    <section v-if="watchMode" class="watch-page">
      <header class="watch-topbar"><button class="icon-button" type="button" title="返回" aria-label="返回" @click="backFromWatch"><ArrowLeft :size="20" /></button><BrandLogo /><span v-if="watchVideo">{{ watchVideo.studio_name }}</span></header>
      <main class="watch-main">
        <div v-if="watchLoading" class="watch-status"><RefreshCw class="spin" :size="24" />正在准备播放</div>
        <div v-else-if="watchError" class="watch-status error"><AlertCircle :size="26" /><strong>{{ watchError }}</strong><button class="secondary" type="button" @click="backFromWatch">返回目录</button></div>
          <template v-else-if="watchVideo && streamURL"><div class="watch-player"><VideoPlayer :src="streamURL" :poster="watchVideo.poster_url" /></div><footer><span>{{ watchVideo.studio_name }}</span><h1>{{ watchVideo.title }}</h1></footer></template>
      </main>
    </section>

    <template v-else>
      <header class="topbar">
        <button class="brand-button" type="button" aria-label="返回首页" @click="openView('home')"><BrandLogo /></button>
        <form class="search-box" @submit.prevent="submitSearch"><Search :size="18" /><input v-model="search" aria-label="搜索视频或工作室" placeholder="搜索视频或工作室" /><button type="submit">搜索</button></form>
        <nav class="desktop-nav" aria-label="主要导航"><button :class="{ active: activeView === 'home' }" @click="openView('home')"><Home :size="17" />首页</button><button :class="{ active: activeView === 'library' }" @click="openView('library')"><Library :size="17" />已购</button><button v-if="account?.is_admin" :class="{ active: activeView === 'admin' }" @click="openView('admin')"><Shield :size="17" />管理</button></nav>
        <button v-if="account" class="account-chip" type="button" @click="openView('account')"><Coins :size="17" /><strong>{{ account.balance }}</strong><span>鹿币</span></button><button v-else class="login-button" type="button" @click="openAuth('login')"><LogIn :size="17" />登录</button>
      </header>

      <main>
        <div v-if="message || error" class="notice-wrap"><button v-if="message" class="notice success" type="button" @click="message = ''"><CheckCircle2 :size="17" />{{ message }}</button><button v-if="error" class="notice error" type="button" @click="error = ''"><AlertCircle :size="17" />{{ error }}</button></div>
        <template v-if="activeView !== 'admin' && activeView !== 'account'">
          <section class="catalog-heading"><div><span>{{ catalogTotal }} 部作品</span><h1>{{ sectionTitle }}</h1></div></section>
          <div v-if="activeView === 'home' && !search.trim()" class="studio-strip"><button :class="{ active: selectedStudio === 0 }" @click="chooseStudio(0)">推荐</button><button v-for="studio in studios" :key="studio.id" :class="{ active: selectedStudio === studio.id }" @click="chooseStudio(studio.id)">{{ studio.name }}<span>{{ studio.video_count }}</span></button></div>
          <section v-if="loading" class="empty-state"><RefreshCw class="spin" :size="24" />正在整理放映单</section><section v-else-if="videos.length" class="video-grid"><VideoCard v-for="(video, index) in videos" :key="video.id" :video="video" :index="index" @open="openVideo" /></section><section v-else class="empty-state"><Film :size="30" /><strong>这里还没有作品</strong><span>媒体节点同步后会自动出现。</span></section>
          <nav v-if="catalogPageCount > 1" class="catalog-pagination" aria-label="视频分页"><button class="icon-button" type="button" :disabled="catalogPage === 1" aria-label="上一页" title="上一页" @click="goToCatalogPage(catalogPage - 1)"><ArrowLeft :size="17" /></button><span>第 {{ catalogPage }} / {{ catalogPageCount }} 页</span><button class="icon-button" type="button" :disabled="catalogPage === catalogPageCount" aria-label="下一页" title="下一页" @click="goToCatalogPage(catalogPage + 1)"><ArrowRight :size="17" /></button></nav>
        </template>

        <section v-else-if="activeView === 'account'" class="account-view">
          <header class="page-heading"><div><span>我的账户</span><h1 class="account-email">{{ account?.email }}</h1></div><button class="icon-text" type="button" @click="logout"><LogOut :size="17" />退出</button></header>
          <div class="account-summary"><div><span>鹿币余额</span><strong>{{ account?.balance ?? 0 }}</strong></div></div>
          <div class="account-columns"><section class="plain-section redeem-section"><header><div><span>兑换码</span><h2>充值鹿币</h2></div></header><p v-if="commerce.redeem_notice" class="redeem-notice"><template v-for="(part, index) in redeemNoticeParts" :key="`${index}-${part.text}`"><a v-if="part.href" :href="part.href" target="_blank" rel="noopener noreferrer">{{ part.text }}</a><span v-else>{{ part.text }}</span></template></p><form class="redeem-form" @submit.prevent="redeem"><input v-model="redeemCode" required placeholder="DEER-XXXX-XXXX-XXXX-XXXX" /><button class="primary" type="submit"><Ticket :size="18" />兑换</button></form></section></div>
          <section class="ledger-section"><header><span>最近记录</span><h2>鹿币明细</h2></header><div class="data-list"><div v-for="entry in walletEntries" :key="entry.id"><span>{{ entry.description }}<small>{{ formatDate(entry.created_at) }}</small></span><strong :class="entry.delta > 0 ? 'positive' : ''">{{ entry.delta > 0 ? '+' : '' }}{{ entry.delta }}</strong></div><p v-if="!walletEntries.length">暂无鹿币记录</p></div></section>
        </section>

        <section v-else class="admin-view">
          <header class="page-heading"><div><span>管理员</span><h1>管理后台</h1></div></header>
          <nav class="admin-tabs"><button v-for="tab in ([['users','用户'],['codes','兑换码'],['nodes','节点']] as const)" :key="tab[0]" :class="{ active: adminTab === tab[0] }" @click="openAdmin(tab[0])">{{ tab[1] }}</button></nav>
          <div v-if="adminTab === 'codes'" class="redeem-admin"><div class="redeem-admin-tools"><form class="admin-form" @submit.prevent="createCodes"><h2>生成兑换码</h2><label>每码鹿币<input v-model.number="codeForm.credits" type="number" min="1" required /></label><label>生成数量<input v-model.number="codeForm.count" type="number" min="1" max="1000" required /></label><button class="primary" type="submit"><Ticket :size="18" />生成并导出 TXT</button></form><form class="admin-form redeem-notice-editor" @submit.prevent="saveRedeemNotice"><h2>兑换获取说明</h2><p>这里的内容会显示在成员充值页面，可填写卡网购买、兑换和输入卡密的说明。</p><textarea v-model="redeemNoticeDraft" maxlength="2000" rows="7" placeholder="例如：请先在卡网兑换，再把得到的卡密粘贴到这里兑换鹿币。"></textarea><button class="secondary" type="submit" :disabled="redeemNoticeSaving">{{ redeemNoticeSaving ? '正在保存' : '保存说明' }}</button></form></div><section class="redeem-records"><header class="admin-section-heading"><div><span>兑换码记录</span><h2>全部兑换</h2></div><strong>{{ redeemTotal }} 条</strong></header><div class="redeem-record-toolbar"><div class="segmented-control"><button v-for="filter in ([['all','全部'],['unused','未使用'],['used','已使用']] as const)" :key="filter[0]" :class="{ active: redeemStatus === filter[0] }" type="button" @click="selectRedeemStatus(filter[0])">{{ filter[1] }} {{ redeemCounts[filter[0]] }}</button></div><form class="admin-code-search" @submit.prevent="submitRedeemSearch"><Search :size="18" /><input v-model="redeemSearch" aria-label="搜索完整卡密或使用者邮箱" placeholder="搜索完整卡密或使用者邮箱" /><button class="secondary" type="submit">搜索</button></form></div><div class="admin-table redeem-table"><div class="table-row redeem-code-row head"><span>编号</span><span>面值</span><span>状态</span><span>使用者</span><span>使用时间</span><span>创建时间</span></div><div v-for="code in redeemCodes" :key="code.id" class="table-row redeem-code-row"><strong>#{{ code.id }}</strong><span>{{ code.credits }} 鹿币</span><span><em :class="code.used ? 'used' : 'unused'">{{ code.used ? '已使用' : '未使用' }}</em></span><span>{{ code.redeemed_by_email || '—' }}</span><span>{{ code.redeemed_at ? formatDate(code.redeemed_at) : '—' }}</span><span>{{ formatDate(code.created_at) }}</span></div><p v-if="!redeemCodes.length" class="table-empty">没有符合条件的兑换码</p></div><nav v-if="redeemPageCount > 1" class="redeem-pagination" aria-label="兑换码分页"><button class="icon-button" type="button" :disabled="redeemPage === 1 || redeemCodesLoading" aria-label="上一页" title="上一页" @click="goToRedeemPage(redeemPage - 1)"><ArrowLeft :size="17" /></button><span>第 {{ redeemPage }} / {{ redeemPageCount }} 页</span><button class="icon-button" type="button" :disabled="redeemPage === redeemPageCount || redeemCodesLoading" aria-label="下一页" title="下一页" @click="goToRedeemPage(redeemPage + 1)"><ArrowRight :size="17" /></button></nav></section></div>
          
          <template v-else-if="adminTab === 'users'"><div class="admin-user-heading"><div><span>成员管理</span><h2>用户账号</h2></div><strong><Users :size="17" />共 {{ userAllTotal }} 个账号</strong></div><form class="admin-user-toolbar" @submit.prevent="submitUserSearch"><Search :size="18" /><input v-model="userSearch" aria-label="搜索用户邮箱" placeholder="搜索用户邮箱" /><button class="secondary" type="submit">搜索</button></form><div class="admin-table"><div class="table-row user-admin head"><span>用户</span><span>鹿币</span><span>操作</span></div><div v-for="user in users" :key="user.id" class="table-row user-admin"><span>{{ user.email }}<small>{{ user.is_admin ? '管理员' : user.enabled ? '正常' : '已停用' }}</small></span><strong>{{ user.balance }}</strong><div class="row-actions"><button class="icon-button" type="button" title="增加鹿币" aria-label="增加鹿币" @click="openCreditAdjustment(user, 1)"><Plus :size="16" /></button><button class="icon-button" type="button" title="扣减鹿币" aria-label="扣减鹿币" @click="openCreditAdjustment(user, -1)"><Minus :size="16" /></button><button class="secondary" :disabled="user.id === account?.id" @click="toggleUser(user)">{{ user.enabled ? '停用' : '启用' }}</button><button class="icon-button" type="button" title="重置密码" aria-label="重置密码" @click="resetUser = user"><KeyRound :size="16" /></button></div></div></div><div v-if="users.length < userTotal" class="load-more-row"><button class="secondary" type="button" :disabled="usersLoadingMore" @click="loadMoreUsers"><RefreshCw v-if="usersLoadingMore" class="spin" :size="17" />{{ usersLoadingMore ? '正在加载' : '加载更多用户' }}</button></div></template>
          <div v-else-if="adminTab === 'nodes'" class="nodes-admin"><header class="admin-section-heading"><div><span>媒体节点</span><h2>节点部署</h2></div><button class="primary" type="button" @click="openNodeCreate"><Plus :size="17" />创建节点</button></header><p class="node-deployment-note">每个节点都有独立身份。导出安装包后只需在 Docker 面板选择一次视频目录；删除节点会清理目录索引、视频权益和播放会话，源视频文件保留。</p><div class="node-list"><article v-for="node in nodes" :key="node.id"><div class="node-heading"><Server :size="22" /><span><strong>{{ node.name }}</strong><small>{{ node.online ? '在线' : '离线' }} · {{ scanStatus(node) }} · {{ formatDate(node.last_seen_at) }}</small><small v-if="node.wireguard_address">WireGuard {{ node.wireguard_address }}</small><small v-if="node.scan_error" class="bad">{{ node.scan_error }}</small></span></div><dl><div><dt>工作室</dt><dd>{{ node.studio_count }}</dd></div><div><dt>视频</dt><dd>{{ node.video_count }}</dd></div><div><dt>总容量</dt><dd>{{ formatBytes(node.total_bytes) }}</dd></div><div><dt>可用容量</dt><dd>{{ formatBytes(node.available_bytes) }}</dd></div></dl><div class="node-actions"><button v-if="node.provisioned && !node.bundle_downloaded && !node.revoked" class="secondary" type="button" :disabled="nodeBundleBusy" @click="downloadNodeBundle(node)"><ArrowDown :size="16" />下载安装包</button><button v-if="node.provisioned && !node.revoked" class="secondary" type="button" :disabled="nodeRotateBusy" @click="rotateNode(node)"><RefreshCw :size="16" />轮换凭据</button><button class="secondary" type="button" :disabled="!node.online || node.scan_status === 'scanning'" @click="rescan(node)"><RefreshCw :size="16" />重新扫描</button><button class="danger-button" type="button" @click="nodeDeleteTarget = node">删除节点</button></div></article><p v-if="!nodes.length" class="table-empty">还没有注册节点</p></div></div>
        </section>
      </main>

      <nav class="mobile-nav" aria-label="移动端导航"><button :class="{ active: activeView === 'home' }" @click="openView('home')"><Home :size="20" /><span>首页</span></button><button :class="{ active: activeView === 'library' }" @click="openView('library')"><Library :size="20" /><span>已购</span></button><button :class="{ active: activeView === 'account' }" @click="openView('account')"><UserIcon :size="20" /><span>我的</span></button><button v-if="account?.is_admin" :class="{ active: activeView === 'admin' }" @click="openView('admin')"><Shield :size="20" /><span>管理</span></button></nav>

      <AppModal v-if="selectedVideo" :title="selectedVideo.title" wide @close="selectedVideo = null"><div class="video-dialog"><div class="player-frame"><img v-if="selectedVideo.poster_url" :src="selectedVideo.poster_url" :alt="selectedVideo.title" /><div v-else class="poster-fallback"><Film :size="42" /></div></div><div class="video-dialog-copy"><span>{{ selectedVideo.studio_name }}</span><h3>{{ selectedVideo.title }}</h3><p>{{ formatBytes(selectedVideo.size_bytes) }} · {{ selectedVideo.width }}×{{ selectedVideo.height }} · {{ selectedVideo.video_codec.toUpperCase() }}</p><div v-if="!selectedVideo.available" class="inline-alert"><AlertCircle :size="18" />媒体节点暂时不可用</div><div v-else-if="selectedVideo.can_play" class="unlock-actions"><button class="primary" :disabled="playerBusy" @click="startPlayback"><Play :size="19" />{{ playerBusy ? '正在准备' : '播放' }}</button><button v-if="!selectedVideo.unlocked" class="secondary" @click="unlockVideo"><Coins :size="19" />{{ commerce.video_price }} 鹿币永久解锁</button></div><div v-else class="unlock-actions"><button class="primary" @click="unlockVideo"><Coins :size="19" />{{ commerce.video_price }} 鹿币永久解锁</button></div></div></div></AppModal>
    </template>

    <AppModal v-if="nodeCreateOpen" title="创建媒体节点" @close="nodeCreateOpen = false"><form class="admin-form" @submit.prevent="provisionNode"><p>创建后会自动分配未使用的 WireGuard 地址，并生成一次性 Docker 安装包。</p><label>节点名称<input v-model="nodeName" pattern="[a-z0-9][a-z0-9._-]{1,62}[a-z0-9]" minlength="3" maxlength="64" placeholder="例如：windows-media" required /></label><button class="primary" type="submit" :disabled="nodeProvisionBusy"><Plus :size="17" />{{ nodeProvisionBusy ? '正在创建' : '创建节点' }}</button></form></AppModal>
    <AppModal v-if="nodeBundleTarget" title="节点安装包" @close="nodeBundleTarget = null"><div class="node-bundle-modal"><CheckCircle2 :size="28" class="good" /><h3>{{ nodeBundleTarget.name }} 已准备好</h3><p>安装包只允许成功下载一次。导入 Docker 项目后，选择一次视频共享目录并启动即可。</p><button class="primary" type="button" :disabled="nodeBundleBusy" @click="downloadNodeBundle(nodeBundleTarget)"><ArrowDown :size="17" />{{ nodeBundleBusy ? '正在下载' : '下载一次性安装包' }}</button></div></AppModal>
    <AppModal v-if="nodeDeleteTarget" title="删除节点" @close="nodeDeleteTarget = null"><div class="node-bundle-modal"><AlertCircle :size="28" class="bad" /><h3>确定删除 {{ nodeDeleteTarget.name }}？</h3><p>节点 Peer、凭据、目录索引、视频权益和播放会话都会删除。NAS 或电脑上的原始视频文件保留，旧安装包立即失效。</p><button class="danger-button danger-wide" type="button" :disabled="nodeDeleteBusy" @click="deleteNode">{{ nodeDeleteBusy ? '正在删除' : '确认删除节点' }}</button></div></AppModal>
    <AppModal v-if="authOpen" :title="authMode === 'login' ? '欢迎回来' : '加入放映室'" @close="authOpen = false"><form class="auth-form" @submit.prevent="submitAuth"><BrandLogo /><p>{{ authMode === 'login' ? '小鹿等你，继续上次的放映。' : '注册后可兑换鹿币、解锁作品。' }}</p><label>邮箱<input v-model="authEmail" type="email" autocomplete="email" required /></label><label>密码<input v-model="authPassword" type="password" minlength="8" maxlength="64" :autocomplete="authMode === 'login' ? 'current-password' : 'new-password'" required /></label><label v-if="authCaptchaRequired" class="captcha-field">验证码<div class="captcha-entry"><input v-model="captchaAnswer" maxlength="5" autocomplete="off" required aria-label="验证码" /><button class="captcha-image" type="button" title="刷新验证码" aria-label="刷新验证码" @click="loadCaptcha"><img v-if="captchaImage" :src="captchaImage" alt="验证码图片" /><RefreshCw v-else :size="19" /></button></div></label><button class="primary" type="submit" :disabled="authBusy || (authCaptchaRequired && !captchaID)">{{ authBusy ? '请稍候' : authMode === 'login' ? '登录' : '注册' }}</button><button class="text-button" type="button" @click="switchAuthMode">{{ authMode === 'login' ? '没有账号？注册' : '已有账号？登录' }}</button></form></AppModal>
    <AppModal v-if="resetUser" title="重置用户密码" @close="resetUser = null; resetPassword = ''"><form class="auth-form" @submit.prevent="submitPasswordReset"><p>{{ resetUser.email }}</p><label>新密码<input v-model="resetPassword" type="password" minlength="8" maxlength="64" required /></label><button class="primary" type="submit"><KeyRound :size="17" />确认重置</button></form></AppModal>
    <AppModal v-if="creditTarget" :title="creditDirection > 0 ? '增加鹿币' : '扣减鹿币'" @close="creditTarget = null"><form class="admin-form" @submit.prevent="submitCreditAdjustment"><p>{{ creditTarget.email }}</p><div class="adjustment-summary"><div><span>当前余额</span><strong>{{ creditTarget.balance }}</strong></div><component :is="creditDirection > 0 ? ArrowUp : ArrowDown" :size="20" /><div><span>调整后</span><strong :class="creditBalanceAfter < 0 ? 'bad' : ''">{{ creditBalanceAfter }}</strong></div></div><label>数量<input v-model.number="creditAmount" type="number" min="1" max="100000" required /></label><label>备注（选填）<input v-model="creditReason" maxlength="200" /></label><button class="primary" type="submit" :disabled="creditBusy || creditBalanceAfter < 0">{{ creditBusy ? '正在提交' : '确认调整' }}</button></form></AppModal>
  </div>
</template>
