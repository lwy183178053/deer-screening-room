<script setup lang="ts">
import Artplayer from 'artplayer'
import { onBeforeUnmount, onMounted, ref } from 'vue'

const props = defineProps<{ src: string; poster?: string }>()
const container = ref<HTMLDivElement | null>(null)
let player: Artplayer | null = null
let wasPlayingBeforeBackground = false
const normalReconnectTimeMax = Artplayer.RECONNECT_TIME_MAX

Artplayer.CONTEXTMENU = false
Artplayer.LOG_VERSION = false

function handleVisibilityChange() {
  if (!player || player.isDestroy) return
  if (document.visibilityState === 'hidden') {
    // Browsers may emit media errors while the page is suspended. Do not spend
    // the normal foreground retry budget before the page can recover itself.
    Artplayer.RECONNECT_TIME_MAX = 0
    wasPlayingBeforeBackground = player.playing
    return
  }
  if (document.visibilityState !== 'visible') return
  Artplayer.RECONNECT_TIME_MAX = normalReconnectTimeMax
  if (!wasPlayingBeforeBackground) return
  wasPlayingBeforeBackground = false
  player.url = props.src
  player.notice.show = false
  player.template.$player.classList.remove('art-error')
  void player.play().catch(() => undefined)
}

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
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  Artplayer.RECONNECT_TIME_MAX = normalReconnectTimeMax
  wasPlayingBeforeBackground = false
  player?.destroy(true)
  player = null
})
</script>

<template>
  <div ref="container" class="deer-player" />
</template>
