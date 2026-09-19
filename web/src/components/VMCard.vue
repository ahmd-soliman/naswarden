<script setup lang="ts">
import { computed } from 'vue'
import type { VM } from '../composables/usePoolSocket'

const props = defineProps<{ vm: VM; active?: boolean }>()
defineEmits<{ select: [] }>()

const isRunning = computed(() => props.vm.status.toLowerCase() === 'running')

const statusBadge = computed(() => {
  const s = props.vm.status.toLowerCase()
  if (s === 'running') return { label: 'Running', cls: 'badge--green' }
  if (s === 'stopped') return { label: 'Stopped', cls: 'badge--gray' }
  if (s === 'frozen') return { label: 'Frozen', cls: 'badge--yellow' }
  return { label: props.vm.status, cls: 'badge--red' }
})

const memPercent = computed(() => {
  if (!isRunning.value || props.vm.mem_total === 0) return 0
  return Math.round((props.vm.mem_used / props.vm.mem_total) * 100)
})

const memColor = computed(() => {
  if (memPercent.value >= 90) return 'red'
  if (memPercent.value >= 70) return 'yellow'
  return 'green'
})

const diskPercent = computed(() => {
  if (props.vm.disk_total === 0) return 0
  return Math.round((props.vm.disk_used / props.vm.disk_total) * 100)
})

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
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
    class="vm-card card--clickable"
    :class="{ 'vm-card--stopped': !isRunning, 'card--open': active }"
    tabindex="0"
    role="button"
    @click="$emit('select')"
    @keydown.enter.prevent="$emit('select')"
    @keydown.space.prevent="$emit('select')"
  >
    <div class="vm-card__header">
      <span class="vm-card__name" :title="vm.name">{{ vm.name }}</span>
      <div class="vm-card__badges">
        <span class="vm-type-pill" :class="vm.is_vm ? 'vm-type-pill--kvm' : 'vm-type-pill--lxc'">
          {{ vm.is_vm ? 'KVM VM' : 'LXC' }}
        </span>
        <span class="badge" :class="statusBadge.cls">
          {{ statusBadge.label }}
        </span>
      </div>
    </div>

    <div class="vm-card__meta">
      <span class="vm-card__os" :title="vm.os || 'Linux'">{{ vm.os || 'Linux' }}</span>
      <span v-if="vm.ipv4.length > 0" class="vm-card__ip">
        {{ vm.ipv4[0] }}<template v-if="vm.ipv4.length > 1"> (+{{ vm.ipv4.length - 1 }})</template>
      </span>
    </div>

    <template v-if="isRunning">
      <div class="vm-card__stat">
        <span class="vm-card__stat-label">
          <svg class="metric-glyph" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M9 1v3M15 1v3M9 20v3M15 20v3M1 9h3M1 15h3M20 9h3M20 15h3"/></svg>
          CPU
        </span>
        <span>
          <template v-if="vm.cpu_cores">{{ vm.cpu_cores }} vCPU · </template>
          {{ vm.cpu_percent.toFixed(1) }}%
        </span>
      </div>
      <div class="vm-card__stat">
        <span class="vm-card__stat-label">
          <svg class="metric-glyph" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="7" width="20" height="10" rx="1"/><path d="M6 7v10M10 7v4M14 7v4M18 7v10"/></svg>
          Memory
        </span>
        <span>{{ formatBytes(vm.mem_used) }} / {{ formatBytes(vm.mem_total) }}</span>
      </div>
      <div class="bar">
        <div class="bar__fill" :class="`badge--${memColor}`" :style="{ width: Math.min(memPercent, 100) + '%' }" />
      </div>

      <div v-if="vm.disk_total > 0" class="vm-card__stat vm-card__stat--disk">
        <span class="vm-card__stat-label">
          <svg class="metric-glyph" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/></svg>
          Disk zvol <template v-if="vm.disk_pool">({{ vm.disk_pool }})</template>
        </span>
        <span>{{ formatBytes(vm.disk_used) }} / {{ formatBytes(vm.disk_total) }} ({{ diskPercent }}%)</span>
      </div>
    </template>
    <div v-else class="vm-card__status">{{ vm.status }}</div>
  </div>
</template>

<style scoped>
.vm-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  --accent: var(--vm);
  border-top: 3px solid var(--accent);
  border-radius: 10px;
  padding: 1rem 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.vm-card--stopped .vm-card__name {
  color: var(--text-dim);
}

.vm-card__header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.4rem 0.75rem;
}

.vm-card__name {
  font-size: 0.9rem;
  font-weight: 600;
  font-family: ui-monospace, monospace;
  overflow-wrap: anywhere;
  min-width: 0;
}

.vm-card__badges {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  flex-shrink: 0;
}

.vm-type-pill {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.03em;
  padding: 0.12rem 0.45rem;
  border-radius: 4px;
  text-transform: uppercase;
  border: 1px solid var(--border);
}

.vm-type-pill--kvm {
  background: rgba(6, 182, 212, 0.15);
  color: #22d3ee;
  border-color: rgba(6, 182, 212, 0.35);
}

.vm-type-pill--lxc {
  background: rgba(107, 114, 128, 0.15);
  color: #cbd5e1;
  border-color: rgba(107, 114, 128, 0.3);
}

.vm-card__meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8rem;
  color: var(--text-dim);
}

.vm-card__os {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.vm-card__ip {
  font-family: ui-monospace, monospace;
  font-size: 0.75rem;
  background: rgba(255, 255, 255, 0.04);
  padding: 0.1rem 0.35rem;
  border-radius: 4px;
  flex-shrink: 0;
}

.badge {
  font-size: 0.8rem;
  font-weight: 600;
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  flex-shrink: 0;
  text-transform: uppercase;
}

.vm-card__stat {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.8rem;
  color: var(--text-dim);
}

.vm-card__stat--disk {
  margin-top: 0.15rem;
  padding-top: 0.3rem;
  border-top: 1px dashed rgba(255, 255, 255, 0.07);
}

.vm-card__stat-label {
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

.vm-card__status {
  font-size: 0.8rem;
  color: var(--text-dim);
}

@media (prefers-reduced-motion: reduce) {
  .bar__fill {
    transition: none;
  }
}
</style>
