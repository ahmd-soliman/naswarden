<script setup lang="ts">
import { computed } from 'vue'
import type { Pool, ReplicationTask } from '../composables/usePoolSocket'

const props = defineProps<{
  pool: Pool
  replications?: ReplicationTask[]
  active?: boolean
}>()
defineEmits<{ select: [] }>()

const targetReplications = computed(() => {
  if (!props.replications) return []
  return props.replications.filter((t) => t.target_pool === props.pool.name)
})

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

function formatRelativeTime(ts: number): string {
  const s = Math.max(0, Math.floor(Date.now() / 1000 - ts))
  if (s < 60) return `${s}s ago`
  if (s < 3600) return `${Math.floor(s / 60)}m ago`
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`
  return `${Math.floor(s / 86400)}d ago`
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

    <div v-if="targetReplications.length > 0" class="pool-card__rep">
      <div v-for="task in targetReplications" :key="task.id" class="pool-card__rep-item">
        <span class="rep-dot" :class="`rep-dot--${(task.job_state || task.state).toLowerCase()}`" />
        <span class="rep-name">{{ task.name }}:</span>
        <span class="rep-state">{{ task.job_state || task.state }}</span>
        <span v-if="task.time_finished" class="rep-time">({{ formatRelativeTime(task.time_finished) }})</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pool-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  --accent: var(--pool);
  border-top: 3px solid var(--accent);
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
  font-size: 0.8rem;
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

.pool-card__rep {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  font-size: 0.8rem;
  color: var(--text-dim);
  border-top: 1px solid var(--border);
  padding-top: 0.5rem;
}

.pool-card__rep-item {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rep-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--text-dim);
  flex-shrink: 0;
}

.rep-dot--success,
.rep-dot--finished {
  background: #22c55e;
}

.rep-dot--running {
  background: #3b82f6;
  animation: pulse 1.5s infinite;
}

.rep-dot--error,
.rep-dot--failed {
  background: #ef4444;
}

.rep-name {
  font-weight: 600;
  color: var(--text);
  font-family: ui-monospace, monospace;
}

.rep-state {
  text-transform: uppercase;
  font-size: 0.7rem;
  font-weight: 600;
}

.rep-time {
  font-family: ui-monospace, monospace;
  font-size: 0.75rem;
  color: var(--text-dim);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

@media (prefers-reduced-motion: reduce) {
  .bar__fill {
    transition: none;
  }
  .rep-dot--running {
    animation: none;
  }
}
</style>
