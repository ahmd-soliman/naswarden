<script setup lang="ts">
import { computed } from 'vue'
import type { ServerInfo } from '../composables/usePoolSocket'

const props = defineProps<{ server: ServerInfo }>()

const memPercent = computed(() => {
  if (props.server.mem_total === 0) return 0
  return Math.round((props.server.mem_used / props.server.mem_total) * 100)
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

function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  if (days > 0) return `${days}d ${hours}h`
  const minutes = Math.floor((seconds % 3600) / 60)
  return `${hours}h ${minutes}m`
}
</script>

<template>
  <div class="server-card">
    <div class="server-card__row">
      <div class="server-card__stat">
        <span class="server-card__label">Host</span>
        <span class="server-card__value">{{ server.hostname }}</span>
      </div>
      <div class="server-card__stat">
        <span class="server-card__label">Version</span>
        <span class="server-card__value">{{ server.version }}</span>
      </div>
      <div class="server-card__stat">
        <span class="server-card__label">Uptime</span>
        <span class="server-card__value">{{ formatUptime(server.uptime_seconds) }}</span>
      </div>
      <div class="server-card__stat">
        <span class="server-card__label">CPU</span>
        <span class="server-card__value">{{ server.cpu_percent.toFixed(0) }}%</span>
      </div>
      <div class="server-card__stat">
        <span class="server-card__label">System pressure (1/5/15m)</span>
        <span class="server-card__value">
          {{ server.load_percent_1.toFixed(0) }}% / {{ server.load_percent_5.toFixed(0) }}% / {{ server.load_percent_15.toFixed(0) }}%
        </span>
      </div>
    </div>

    <div class="server-card__mem">
      <div class="server-card__mem-header">
        <span class="server-card__label">Memory</span>
        <span class="server-card__value">
          {{ formatBytes(server.mem_used) }} / {{ formatBytes(server.mem_total) }} ({{ memPercent }}%)
        </span>
      </div>
      <div class="bar">
        <div class="bar__fill" :class="`badge--${memColor}`" :style="{ width: Math.min(memPercent, 100) + '%' }" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.server-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.server-card__row {
  display: flex;
  flex-wrap: wrap;
  gap: 1.5rem;
}

.server-card__stat {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.server-card__label {
  font-size: 0.7rem;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.server-card__value {
  font-size: 0.9rem;
  font-family: ui-monospace, monospace;
}

.server-card__mem {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.server-card__mem-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
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

.bar__fill.badge--green {
  background: #22c55e;
}
.bar__fill.badge--yellow {
  background: #eab308;
}
.bar__fill.badge--red {
  background: #ef4444;
}
</style>
