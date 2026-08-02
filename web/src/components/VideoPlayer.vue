<script setup lang="ts">
import Artplayer from 'artplayer'
import { onBeforeUnmount, onMounted, ref } from 'vue'

const props = defineProps<{ src: string; poster?: string }>()
const container = ref<HTMLDivElement | null>(null)
let player: Artplayer | null = null

Artplayer.CONTEXTMENU = false
Artplayer.LOG_VERSION = false

onMounted(() => {
  if (!container.value) return
  player = new Artplayer({
    container: container.value,
    url: props.src,
    poster: props.poster ?? '',
    theme: '#e7655b',
    lang: 'zh-cn',
    autoplay: true,
    volume: 0.7,
    setting: true,
    playbackRate: true,
    pip: true,
    fullscreen: true,
    fullscreenWeb: true,
    hotkey: true,
    mutex: true,
    backdrop: true,
    miniProgressBar: true,
    playsInline: true,
    lock: true,
    gesture: true,
    fastForward: true,
    autoOrientation: true,
    screenshot: false,
    autoPlayback: false,
    airplay: false,
    moreVideoAttr: { preload: 'metadata', playsInline: true },
  })
  player.contextmenu.remove('version')
})

onBeforeUnmount(() => {
  player?.destroy(true)
  player = null
})
</script>

<template>
  <div ref="container" class="deer-player" />
</template>
