import { nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp } from 'vue'
import Artplayer from 'artplayer'
import VideoPlayer from './VideoPlayer.vue'

const { MockArtplayer, players } = vi.hoisted(() => {
  const instances: MockArtplayer[] = []
  class MockArtplayer {
    static CONTEXTMENU = true
    static LOG_VERSION = true
    static RECONNECT_TIME_MAX = 5
    static RECONNECT_SLEEP_TIME = 1000
    readonly contextmenu = { remove: vi.fn() }
    readonly option: Record<string, unknown>
    readonly play = vi.fn(async () => undefined)
    readonly destroy = vi.fn()
    readonly notice = { show: false }
    readonly template = { $player: document.createElement('div') }
    url = ''
    isDestroy = false
    playing = true

    constructor(option: Record<string, unknown>) {
      this.option = option
      this.url = String(option.url ?? '')
      instances.push(this)
    }
  }
  return { MockArtplayer, players: instances }
})

vi.mock('artplayer', () => ({ default: MockArtplayer }))

function setVisibility(state: 'hidden' | 'visible') {
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: state })
  document.dispatchEvent(new Event('visibilitychange'))
}

describe('VideoPlayer', () => {
  beforeEach(() => {
    players.length = 0
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('recovers once when the page returns from the background', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const app = createApp(VideoPlayer, { src: '/api/v1/playback/session/stream' })
    app.mount(container)
    await nextTick()

    const player = players[0]
    setVisibility('hidden')
    expect(Artplayer.RECONNECT_TIME_MAX).toBe(0)
    setVisibility('visible')
    await nextTick()

    expect(player.url).toBe('/api/v1/playback/session/stream')
    expect(player.play).toHaveBeenCalledTimes(1)
    expect(Artplayer.RECONNECT_TIME_MAX).toBe(5)

    app.unmount()
    container.remove()
  })

  it('does not recover after the player is unmounted', async () => {
    vi.useFakeTimers()
    const container = document.createElement('div')
    document.body.append(container)
    const app = createApp(VideoPlayer, { src: '/stream' })
    app.mount(container)
    await nextTick()
    const player = players[0]
    app.unmount()

    setVisibility('hidden')
    setVisibility('visible')
    vi.runAllTimers()

    expect(player.play).not.toHaveBeenCalled()
    container.remove()
  })
})
