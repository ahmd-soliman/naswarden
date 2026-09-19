<script setup lang="ts">
import { computed } from 'vue'
import type { Pool } from '../composables/usePoolSocket'

const props = defineProps<{ pool: Pool; active?: boolean }>()
defineEmits<{ select: [] }>()

const statusColor = computed(() => {
  if (!props.pool.healthy) return 'red'
  if (props.pool.warning) return 'yellow'
  return 'green'
})

const usedPercent = computed(() => {
  if (props.pool.size === 0) return 0
  return Math.round((props.pool.allocated / props.pool.size) * 100)
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
  <div
    class="pool-card card--clickable"
    :class="{ 'card--open': active }"
    tabindex="0"
    role="button"
    @click="$emit('select')"
    @keydown.enter.prevent="$emit('select')"
    @keydown.space.prevent="$emit('select')"
  >
    <div class="pool-card__header">
      <span class="pool-card__name">{{ pool.name }}</span>
      <span class="badge" :class="`badge--${statusColor}`">{{ pool.status }}</span>
    </div>

    <div class="pool-card__usage">
      <div class="bar">
        <div class="bar__fill" :class="`badge--${statusColor}`" :style="{ width: usedPercent + '%' }" />
      </div>
      <span class="pool-card__usage-text">
        {{ formatBytes(pool.allocated) }} / {{ formatBytes(pool.size) }} ({{ usedPercent }}%)
      </span>
    </div>

    <div v-if="pool.scan" class="pool-card__scan">
      Last {{ pool.scan.function.toLowerCase() }}: {{ pool.scan.state.toLowerCase() }}
      <span v-if="pool.scan.errors > 0" class="pool-card__scan-errors">
        ({{ pool.scan.errors }} errors)
      </span>
    </div>
  </div>
</template>

<style scoped>
.pool-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-top: 3px solid var(--pool);
  border-radius: 10px;
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.pool-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.pool-card__name {
  font-size: 1.1rem;
  font-weight: 600;
  font-family: ui-monospace, monospace;
}

.badge {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.2rem 0.6rem;
  border-radius: 999px;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.pool-card__usage {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.bar {
  height: 8px;
  background: var(--border);
  border-radius: 999px;
  overflow: hidden;
}

.bar__fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.4s ease;
}

.pool-card__usage-text {
  font-size: 0.85rem;
  color: var(--text-dim);
}

.pool-card__scan {
  font-size: 0.85rem;
  color: var(--text-dim);
}

.pool-card__scan-errors {
  color: var(--crit-t);
  font-weight: 600;
}

@media (prefers-reduced-motion: reduce) {
  .bar__fill {
    transition: none;
  }
}
</style>
