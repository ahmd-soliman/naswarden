<script setup lang="ts">
import { computed } from 'vue'
import type { VM } from '../composables/usePoolSocket'

const props = defineProps<{ vm: VM }>()

function formatStartedAt(iso: string): string {
  if (!iso) return 'unknown'
  return new Date(iso).toLocaleString()
}

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

const statusBadge = computed(() => {
  const s = props.vm.status.toLowerCase()
  if (s === 'running') return { label: 'Running', cls: 'badge--green' }
  if (s === 'stopped') return { label: 'Stopped', cls: 'badge--gray' }
  if (s === 'frozen') return { label: 'Frozen', cls: 'badge--yellow' }
  return { label: props.vm.status, cls: 'badge--red' }
})

// Filter config to interesting operational keys, excluding bulky scripts
const filteredConfig = computed(() => {
  if (!props.vm.config) return []
  const entries: { key: string; value: string }[] = []
  for (const [k, v] of Object.entries(props.vm.config)) {
    if (k.includes('user-data') || k.includes('network-config')) continue
    entries.push({ key: k, value: v })
  }
  return entries.sort((a, b) => a.key.localeCompare(b.key))
})
</script>

<template>
  <div class="drawer__section">
    <h3>Overview</h3>
    <div class="drawer__kv">
      <span>Type</span>
      <span class="vm-details-type">
        <span class="vm-type-pill" :class="vm.is_vm ? 'vm-type-pill--kvm' : 'vm-type-pill--lxc'">
          {{ vm.is_vm ? 'KVM Virtual Machine' : 'LXC Container' }}
        </span>
      </span>
    </div>
    <div class="drawer__kv">
      <span>Status</span>
      <span class="badge" :class="statusBadge.cls">{{ statusBadge.label }}</span>
    </div>
    <div class="drawer__kv"><span>Operating System</span><span>{{ vm.os || 'Linux' }}</span></div>
    <div v-if="vm.kernel" class="drawer__kv"><span>Kernel</span><span>{{ vm.kernel }}</span></div>
    <div v-if="vm.arch" class="drawer__kv"><span>Architecture</span><span>{{ vm.arch }}</span></div>
    <div class="drawer__kv"><span>Started</span><span>{{ formatStartedAt(vm.started_at) }}</span></div>
    <div class="drawer__kv"><span>Auto-start on boot</span><span>{{ vm.auto_start ? 'Yes (boot.autostart)' : 'No' }}</span></div>
  </div>

  <div class="drawer__section">
    <h3>Resources</h3>
    <div class="drawer__kv">
      <span>vCPU Allocation</span>
      <span>{{ vm.cpu_cores > 0 ? `${vm.cpu_cores} vCPUs` : 'Shared' }}</span>
    </div>
    <div class="drawer__kv">
      <span>Live CPU Usage</span>
      <span>{{ vm.cpu_percent.toFixed(1) }}%</span>
    </div>
    <div class="drawer__kv">
      <span>Memory Usage</span>
      <span>{{ formatBytes(vm.mem_used) }} / {{ formatBytes(vm.mem_total) }}</span>
    </div>
    <div v-if="vm.disk_total > 0" class="drawer__kv">
      <span>Root Storage zvol</span>
      <span>{{ formatBytes(vm.disk_used) }} / {{ formatBytes(vm.disk_total) }} (pool: {{ vm.disk_pool || 'P2' }})</span>
    </div>
  </div>

  <div class="drawer__section">
    <h3>Networking</h3>
    <div v-if="vm.bridge" class="drawer__kv">
      <span>Bridge</span>
      <span>{{ vm.bridge }}</span>
    </div>
    <div v-if="vm.mac" class="drawer__kv">
      <span>MAC Address</span>
      <span class="drawer__mono">{{ vm.mac }}</span>
    </div>
    <div class="drawer__kv">
      <span>Guest IPv4</span>
      <span v-if="vm.ipv4.length === 0" class="drawer__empty-inline">None detected</span>
      <span v-else class="drawer__mono">{{ vm.ipv4.join(', ') }}</span>
    </div>
  </div>

  <div v-if="filteredConfig.length > 0" class="drawer__section">
    <h3>Configuration</h3>
    <div v-for="cfg in filteredConfig" :key="cfg.key" class="drawer__kv">
      <span class="drawer__mono-key">{{ cfg.key }}</span>
      <span class="drawer__mono">{{ cfg.value }}</span>
    </div>
  </div>
</template>

<style scoped>
.vm-details-type {
  display: flex;
  align-items: center;
}

.vm-type-pill {
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.03em;
  padding: 0.15rem 0.55rem;
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

.drawer__mono {
  font-family: ui-monospace, monospace;
  font-size: 0.85rem;
}

.drawer__mono-key {
  font-family: ui-monospace, monospace;
  font-size: 0.8rem;
  color: var(--text-dim);
}

.drawer__empty-inline {
  color: var(--text-dim);
  font-style: italic;
}
</style>
