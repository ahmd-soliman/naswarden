<script setup lang="ts">
import { computed } from 'vue'
import type { Container } from '../composables/usePoolSocket'

const props = defineProps<{ container: Container; active?: boolean }>()
defineEmits<{ select: [] }>()

const isRunning = computed(() => props.container.state === 'running')

const memPercent = computed(() => {
  if (!isRunning.value || props.container.mem_limit === 0) return 0
  return Math.round((props.container.mem_used / props.container.mem_limit) * 100)
})

const memColor = computed(() => {
  if (memPercent.value >= 90) return 'red'
  if (memPercent.value >= 70) return 'yellow'
  return 'green'
})

function formatBytes(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex++
  }
  return `${value.toFixed(1)} ${units[unitIndex]}`
}
</script>

<template>
  <div
    class="container-card card--clickable"
    :class="{ 'container-card--stopped': !isRunning, 'card--open': active }"
    tabindex="0"
    role="button"
    @click="$emit('select')"
    @keydown.enter.prevent="$emit('select')"
    @keydown.space.prevent="$emit('select')"
  >
    <div class="container-card__header">
      <span class="container-card__name">{{ container.name }}</span>
      <span class="badge" :class="isRunning ? 'badge--green' : 'badge--gray'">
        {{ container.state }}
      </span>
    </div>

    <div class="container-card__image">{{ container.image }}</div>

    <template v-if="isRunning">
      <div class="container-card__stat">
        <span class="container-card__stat-label">
          <svg class="metric-glyph" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M9 1v3M15 1v3M9 20v3M15 20v3M1 9h3M1 15h3M20 9h3M20 15h3"/></svg>
          CPU
        </span>
        <span>{{ container.cpu_percent.toFixed(1) }}%</span>
      </div>
      <div class="container-card__stat">
        <span class="container-card__stat-label">
          <svg class="metric-glyph" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="7" width="20" height="10" rx="1"/><path d="M6 7v10M10 7v4M14 7v4M18 7v10"/></svg>
          Memory
        </span>
        <span>{{ formatBytes(container.mem_used) }} / {{ formatBytes(container.mem_limit) }}</span>
      </div>
      <div class="bar">
        <div class="bar__fill" :class="`badge--${memColor}`" :style="{ width: Math.min(memPercent, 100) + '%' }" />
      </div>
    </template>
    <div v-else class="container-card__status">{{ container.status }}</div>
  </div>
</template>

<style scoped>
.container-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-top: 3px solid var(--container);
  border-radius: 10px;
  padding: 1rem 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.container-card--stopped {
  opacity: 0.6;
}

.container-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.container-card__name {
  font-size: 0.9rem;
  font-weight: 600;
  font-family: ui-monospace, monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.container-card__image {
  font-size: 0.75rem;
  color: var(--text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.badge {
  font-size: 0.7rem;
  font-weight: 600;
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  flex-shrink: 0;
  text-transform: uppercase;
}

.container-card__stat {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.8rem;
  color: var(--text-dim);
}

.container-card__stat-label {
  display: flex;
  align-items: center;
}

.metric-glyph {
  width: 14px;
  height: 14px;
  color: var(--text-dim);
  flex-shrink: 0;
  margin-right: 0.3rem;
}

.bar {
  height: 6px;
  background: var(--border);
  border-radius: 999px;
  overflow: hidden;
}

.bar__fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.4s ease;
}

.container-card__status {
  font-size: 0.8rem;
  color: var(--text-dim);
}

@media (prefers-reduced-motion: reduce) {
  .bar__fill {
    transition: none;
  }
}
</style>
