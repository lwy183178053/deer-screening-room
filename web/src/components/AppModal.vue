<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { X } from '@lucide/vue'

const props = defineProps<{ title: string; wide?: boolean }>()
const emit = defineEmits<{ close: [] }>()
function keydown(event: KeyboardEvent) { if (event.key === 'Escape') emit('close') }
onMounted(() => document.addEventListener('keydown', keydown))
onUnmounted(() => document.removeEventListener('keydown', keydown))
</script>

<template>
  <div class="modal-backdrop" role="presentation" @mousedown.self="emit('close')">
    <section class="modal" :class="{ wide }" role="dialog" aria-modal="true" :aria-label="props.title">
      <header><h2>{{ title }}</h2><button class="icon-button" type="button" aria-label="关闭" title="关闭" @click="emit('close')"><X :size="19" /></button></header>
      <div class="modal-body"><slot /></div>
    </section>
  </div>
</template>

