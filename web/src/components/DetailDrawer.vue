<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'

const props = defineProps<{ open: boolean; title: string }>()
const emit = defineEmits<{ close: [] }>()

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.open) emit('close')
}

onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="drawer-backdrop" :class="{ open }" @click="emit('close')" />
  <div class="drawer" :class="{ open }" role="dialog" aria-modal="true">
    <div class="drawer__header">
      <span class="drawer__title">{{ title }}</span>
      <button class="drawer__close" aria-label="Close" @click="emit('close')">&times;</button>
    </div>
    <div class="drawer__body">
      <slot />
    </div>
  </div>
</template>
