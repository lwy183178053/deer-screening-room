<script setup lang="ts">
import { LockKeyhole, Play } from '@lucide/vue'
import type { Video } from '../types'
import { formatDuration } from '../format'

defineProps<{ video: Video; index?: number }>()
const emit = defineEmits<{ open: [video: Video] }>()
</script>

<template>
  <button class="video-card" type="button" :style="{ '--index': index ?? 0 }" @click="emit('open', video)">
    <span class="poster-frame">
      <img v-if="video.poster_url" :src="video.poster_url" :alt="video.title" loading="lazy" />
      <span v-else class="poster-fallback"><Play :size="28" /></span>
      <span class="duration">{{ formatDuration(video.duration_ms) }}</span>
      <span v-if="!video.available" class="availability">暂不可用</span>
      <span v-else-if="!video.can_play" class="locked"><LockKeyhole :size="14" /></span>
    </span>
    <span class="video-copy"><strong>{{ video.title }}</strong><span>{{ video.studio_name }}</span></span>
  </button>
</template>

