<script setup lang="ts">
import { computed } from 'vue'
import type { Container } from '../composables/usePoolSocket'

const props = defineProps<{ container: Container }>()

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
  <div class="container-card" :class="{ 'container-card--stopped': !isRunning }">
    <div class="container-card__header">
      <span class="container-card__name">{{ container.name }}</span>
      <span class="badge" :class="isRunning ? 'badge--green' : 'badge--gray'">
        {{ container.state }}
      </span>
    </div>

    <div class="container-card__image">{{ container.image }}</div>

    <template v-if="isRunning">
      <div class="container-card__stat">
        <span>CPU</span>
        <span>{{ container.cpu_percent.toFixed(1) }}%</span>
      </div>
      <div class="container-card__stat">
        <span>Memory</span>
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

.badge--green {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
}

.badge--gray {
  background: rgba(156, 163, 175, 0.15);
  color: #9ca3af;
}

.badge--yellow {
  background: rgba(234, 179, 8, 0.15);
  color: #eab308;
}

.badge--red {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.container-card__stat {
  display: flex;
  justify-content: space-between;
  font-size: 0.8rem;
  color: var(--text-dim);
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

.bar__fill.badge--green {
  background: #22c55e;
}
.bar__fill.badge--yellow {
  background: #eab308;
}
.bar__fill.badge--red {
  background: #ef4444;
}

.container-card__status {
  font-size: 0.8rem;
  color: var(--text-dim);
}
</style>
