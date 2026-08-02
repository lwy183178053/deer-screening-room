<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from '../api'
import type { IceServer } from '../types'
import 'media-chrome'

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
    const timer = window.setTimeout(finish, 1500)
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
  await peer.setLocalDescription(offer)
  const iceComplete = waitForICE(current)
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
  <media-controller class="media-player" defaultstreamtype="on-demand">
    <video ref="video" slot="media" class="media-player-video native-player" autoplay playsinline preload="metadata" />
    <media-control-bar>
      <media-play-button></media-play-button>
      <media-mute-button></media-mute-button>
      <media-volume-range></media-volume-range>
      <media-time-range></media-time-range>
      <media-time-display showduration></media-time-display>
      <media-playback-rate-button></media-playback-rate-button>
      <media-fullscreen-button></media-fullscreen-button>
    </media-control-bar>
  </media-controller>
</template>
