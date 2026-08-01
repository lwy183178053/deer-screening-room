<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from '../api'
import type { IceServer } from '../types'

const props = defineProps<{ sessionId: string; iceServers: IceServer[]; p2pEnabled: boolean }>()
const emit = defineEmits<{ status: [value: 'connecting' | 'direct' | 'turn' | 'failed']; error: [value: string] }>()
const video = ref<HTMLVideoElement | null>(null)
const playbackRate = ref(1)
let peer: RTCPeerConnection | null = null
let closed = false

function waitForICE(current: RTCPeerConnection) {
  if (current.iceGatheringState === 'complete') return Promise.resolve()
  return new Promise<void>(resolve => {
    const finish = () => { current.onicegatheringstatechange = null; resolve() }
    const timer = window.setTimeout(finish, 5000)
    current.onicegatheringstatechange = () => {
      if (current.iceGatheringState === 'complete') { window.clearTimeout(timer); finish() }
    }
  })
}

async function reportConnectionPath(current: RTCPeerConnection) {
  const stats = await current.getStats()
  let relay = !props.p2pEnabled
  stats.forEach(report => {
    if (report.type !== 'candidate-pair' || report.state !== 'succeeded' || report.nominated === false) return
    const local = stats.get(report.localCandidateId)
    const remote = stats.get(report.remoteCandidateId)
    relay = relay || local?.candidateType === 'relay' || remote?.candidateType === 'relay'
  })
  emit('status', relay ? 'turn' : 'direct')
}

async function connect() {
  emit('status', 'connecting')
  peer = new RTCPeerConnection({ iceServers: props.iceServers, iceTransportPolicy: props.p2pEnabled ? 'all' : 'relay' })
  const current = peer
  peer.addTransceiver('video', { direction: 'recvonly' })
  peer.addTransceiver('audio', { direction: 'recvonly' })
  peer.ontrack = event => {
    if (video.value && event.streams[0]) { video.value.srcObject = event.streams[0]; void video.value.play().catch(() => undefined) }
  }
  peer.onconnectionstatechange = () => {
    if (closed) return
    if (current.connectionState === 'connected') void reportConnectionPath(current)
    if (current.connectionState === 'failed') { emit('status', 'failed'); emit('error', 'WebRTC 播放连接失败') }
  }
  const offer = await peer.createOffer()
  const iceComplete = waitForICE(current)
  await peer.setLocalDescription(offer)
  await iceComplete
  if (!peer.localDescription) throw new Error('WebRTC offer 创建失败')
  const answer = await api<{ sdp: string; type: RTCSdpType }>(`/api/v1/p2p/${props.sessionId}/offer`, { method: 'POST', body: JSON.stringify({ sdp: peer.localDescription.sdp, type: peer.localDescription.type }) })
  if (closed || !peer) return
  await peer.setRemoteDescription(answer)
}

watch(playbackRate, value => { if (video.value) video.value.playbackRate = value })
onMounted(() => { void connect().catch(error => { if (!closed) { emit('status', 'failed'); emit('error', error instanceof Error ? error.message : 'WebRTC 播放失败') } }) })
onBeforeUnmount(() => { closed = true; if (video.value) video.value.srcObject = null; peer?.close(); peer = null; void api(`/api/v1/p2p/${props.sessionId}`, { method: 'DELETE' }).catch(() => undefined) })
</script>

<template>
  <div class="native-player-shell">
    <video ref="video" class="native-player" controls autoplay playsinline preload="metadata" />
    <div class="native-player-tools">
      <label>播放速度<select v-model.number="playbackRate" aria-label="播放速度"><option :value="0.75">0.75x</option><option :value="1">1x</option><option :value="1.25">1.25x</option><option :value="1.5">1.5x</option><option :value="2">2x</option></select></label>
    </div>
  </div>
</template>
