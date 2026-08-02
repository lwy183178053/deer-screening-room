import { createApp, nextTick } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import VideoPlayer from './VideoPlayer.vue'

class PeerConnectionMock {
  static instances: PeerConnectionMock[] = []
  iceGatheringState: RTCIceGatheringState = 'complete'
  connectionState: RTCPeerConnectionState = 'new'
  localDescription: RTCSessionDescriptionInit | null = null
  ontrack: ((event: RTCTrackEvent) => void) | null = null
  onconnectionstatechange: (() => void) | null = null
  closed = false

  constructor(readonly configuration: RTCConfiguration) { PeerConnectionMock.instances.push(this) }
  addTransceiver() { return {} as RTCRtpTransceiver }
  async createOffer() { return { type: 'offer' as RTCSdpType, sdp: 'browser-offer' } }
  async setLocalDescription(value: RTCSessionDescriptionInit) { this.localDescription = value }
  async setRemoteDescription() {
    this.connectionState = 'connected'
    this.onconnectionstatechange?.()
  }
  addEventListener() {}
  async getStats() {
    return new Map<string, RTCStats>([
      ['pair', { id: 'pair', type: 'candidate-pair', timestamp: 0, state: 'succeeded', nominated: true, localCandidateId: 'local' } as RTCStats],
      ['local', { id: 'local', type: 'local-candidate', timestamp: 0, candidateType: 'relay' } as RTCStats],
    ]) as RTCStatsReport
  }
  close() { this.closed = true; this.connectionState = 'closed' }
}

describe('VideoPlayer', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    PeerConnectionMock.instances = []
  })

  it('uses relay-only signaling and closes both ends of the session', async () => {
    const requests: Array<{ path: string; method: string; body?: Record<string, unknown> }> = []
    vi.stubGlobal('RTCPeerConnection', PeerConnectionMock)
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue()
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      requests.push({ path: String(input), method: init?.method ?? 'GET', body: init?.body ? JSON.parse(String(init.body)) : undefined })
      if (init?.method === 'DELETE') return new Response(null, { status: 204 })
      return new Response(JSON.stringify({ sdp: 'node-answer', type: 'answer' }), { status: 200, headers: { 'Content-Type': 'application/json' } })
    }))

    const container = document.createElement('div')
    document.body.append(container)
    const statuses: string[] = []
    const app = createApp(VideoPlayer, {
      sessionId: 'session-id',
      iceServers: [{ urls: ['turn:turn.example.test:3478'], username: 'user', credential: 'secret' }],
      p2pEnabled: false,
      onStatus: (value: string) => statuses.push(value),
    })
    app.mount(container)
    for (let index = 0; index < 8; index += 1) { await Promise.resolve(); await nextTick() }

    const peer = PeerConnectionMock.instances[0]
    expect(peer.configuration.iceTransportPolicy).toBe('relay')
    expect(requests[0]).toMatchObject({ path: '/api/v1/p2p/session-id/offer', method: 'POST', body: { sdp: 'browser-offer', type: 'offer' } })
    expect(requests[0].body).not.toHaveProperty('ice_servers')
    expect(statuses).toContain('turn')

    app.unmount()
    await Promise.resolve()
    expect(peer.closed).toBe(true)
    expect(requests.some(request => request.path === '/api/v1/p2p/session-id' && request.method === 'DELETE')).toBe(true)
    container.remove()
  })

  it('uses direct ICE policy when enabled and renders Media Chrome controls', async () => {
    vi.stubGlobal('RTCPeerConnection', PeerConnectionMock)
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue()
    vi.stubGlobal('fetch', vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === 'DELETE') return new Response(null, { status: 204 })
      return new Response(JSON.stringify({ sdp: 'node-answer', type: 'answer' }), { status: 200 })
    }))
    const container = document.createElement('div')
    document.body.append(container)
    const app = createApp(VideoPlayer, { sessionId: 'direct', iceServers: [{ urls: ['stun:stun.example.test:3478'] }], p2pEnabled: true })
    app.mount(container)
    for (let index = 0; index < 8; index += 1) { await Promise.resolve(); await nextTick() }
    expect(PeerConnectionMock.instances[0].configuration.iceTransportPolicy).toBe('all')
    expect(container.querySelector('media-controller')).not.toBeNull()
    expect(container.querySelector('media-control-bar')).not.toBeNull()
    app.unmount()
    container.remove()
  })
})
