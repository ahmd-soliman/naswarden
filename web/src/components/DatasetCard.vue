<script setup lang="ts">
import { computed } from 'vue'
import type { Dataset } from '../composables/usePoolSocket'

const props = defineProps<{ dataset: Dataset }>()

const usedPercent = computed(() => {
  if (props.dataset.quota === 0) return 0
  return Math.round((props.dataset.used / props.dataset.quota) * 100)
})

const statusColor = computed(() => {
  if (usedPercent.value >= 90) return 'red'
  if (usedPercent.value >= 70) return 'yellow'
  return 'green'
})

function formatBytes(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
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
  <div class="dataset-card">
    <div class="dataset-card__header">
      <span class="dataset-card__name">{{ dataset.name }}</span>
      <span class="badge" :class="`badge--${statusColor}`">{{ usedPercent }}%</span>
    </div>

    <div class="bar">
      <div class="bar__fill" :class="`badge--${statusColor}`" :style="{ width: Math.min(usedPercent, 100) + '%' }" />
    </div>

    <div class="dataset-card__usage-text">
      {{ formatBytes(dataset.used) }} / {{ formatBytes(dataset.quota) }}
      <span class="dataset-card__source">({{ dataset.quota_source }})</span>
    </div>
  </div>
</template>

<style scoped>
.dataset-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-top: 3px solid var(--dataset);
  border-radius: 10px;
  padding: 1rem 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.dataset-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.dataset-card__name {
  font-size: 0.9rem;
  font-weight: 600;
  font-family: ui-monospace, monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.badge {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.2rem 0.6rem;
  border-radius: 999px;
  flex-shrink: 0;
}

.badge--green {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
}

.badge--yellow {
  background: rgba(234, 179, 8, 0.15);
  color: #eab308;
}

.badge--red {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
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

.dataset-card__usage-text {
  font-size: 0.8rem;
  color: var(--text-dim);
}

.dataset-card__source {
  opacity: 0.7;
}

@media (prefers-reduced-motion: reduce) {
  .bar__fill {
    transition: none;
  }
}
</style>
