<script setup lang="ts">
import deerMark from '../assets/deer-mark.svg'

defineProps<{ compact?: boolean }>()
</script>

<template>
  <div class="brand-logo" :class="{ compact }">
    <img :src="deerMark" alt="" />
    <div v-if="!compact" class="brand-copy"><strong>小鹿放映室</strong><span>小鹿等你</span></div>
  </div>
</template>

