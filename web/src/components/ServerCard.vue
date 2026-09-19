<script setup lang="ts">
import { computed } from 'vue'
import type { ServerInfo } from '../composables/usePoolSocket'

const props = defineProps<{ server: ServerInfo; active?: boolean }>()
defineEmits<{ select: [] }>()

const memPercent = computed(() => {
  if (props.server.mem_total === 0) return 0
  return Math.round((props.server.mem_used / props.server.mem_total) * 100)
})

const memColor = computed(() => {
  if (memPercent.value >= 90) return 'red'
  if (memPercent.value >= 70) return 'yellow'
  return 'green'
})

// Thresholds are typical for a desktop-class AMD/Intel CPU under a NAS
// workload (idle sits well under 60C) -- not a datasheet Tjmax figure.
const tempColor = computed(() => {
  if (props.server.cpu_temp_c >= 85) return 'red'
  if (props.server.cpu_temp_c >= 70) return 'yellow'
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
  <div
    class="server-card card--clickable"
    :class="{ 'card--open': active }"
    tabindex="0"
    role="button"
    @click="$emit('select')"
    @keydown.enter.prevent="$emit('select')"
    @keydown.space.prevent="$emit('select')"
  >
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
        <span class="server-card__label">
          <svg class="metric-glyph" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M9 1v3M15 1v3M9 20v3M15 20v3M1 9h3M1 15h3M20 9h3M20 15h3"/></svg>
          CPU
        </span>
        <span class="server-card__value">{{ server.cpu_percent.toFixed(0) }}%</span>
      </div>
      <div class="server-card__stat">
        <span class="server-card__label">
          <svg class="metric-glyph" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 4v10.54a4 4 0 1 1-4 0V4a2 2 0 0 1 4 0z"/></svg>
          Temp
        </span>
        <span class="server-card__value" :class="`temp--${tempColor}`">{{ server.cpu_temp_c.toFixed(0) }}&deg;C</span>
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
        <span class="server-card__label">
          <svg class="metric-glyph" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="7" width="20" height="10" rx="1"/><path d="M6 7v10M10 7v4M14 7v4M18 7v10"/></svg>
          Memory
        </span>
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

.server-card__value {
  font-size: 0.9rem;
  font-family: ui-monospace, monospace;
}

.temp--green {
  color: var(--ok-t);
}
.temp--yellow {
  color: var(--warn-t);
}
.temp--red {
  color: var(--crit-t);
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

@media (prefers-reduced-motion: reduce) {
  .bar__fill {
    transition: none;
  }
}
</style>
