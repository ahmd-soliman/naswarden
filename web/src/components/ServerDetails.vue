<script setup lang="ts">
import { computed } from 'vue'
import type { ServerInfo } from '../composables/usePoolSocket'

const props = defineProps<{ server: ServerInfo }>()

// mem_used includes ARC (ZFS's disk cache) -- split it out since ARC is
// reclaimable cache, not memory actually held by applications, and this
// is the single biggest reason "used" memory looks alarming on a NAS.
const appUsed = computed(() => Math.max(props.server.mem_used - props.server.arc_bytes, 0))
const free = computed(() => Math.max(props.server.mem_total - props.server.mem_used, 0))

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
  <div class="drawer__section">
    <h3>Processor</h3>
    <div class="drawer__kv"><span>Model</span><span>{{ server.cpu_model }}</span></div>
    <div class="drawer__kv"><span>Physical cores</span><span>{{ server.physical_cores }}</span></div>
    <div class="drawer__kv"><span>Logical cores (threads)</span><span>{{ server.cores }}</span></div>
  </div>

  <div class="drawer__section">
    <h3>Memory breakdown</h3>
    <div class="drawer__kv"><span>ZFS ARC cache</span><span>{{ formatBytes(server.arc_bytes) }}</span></div>
    <div class="drawer__kv"><span>Used</span><span>{{ formatBytes(appUsed) }}</span></div>
    <div class="drawer__kv"><span>Free</span><span>{{ formatBytes(free) }}</span></div>
  </div>

  <div class="drawer__section">
    <h3>Network interfaces</h3>
    <div v-if="server.interfaces.length === 0" class="drawer__empty">No active interfaces.</div>
    <div v-for="iface in server.interfaces" :key="iface.name" class="drawer__mount">
      <strong>{{ iface.name }}</strong> -- {{ iface.speed }}<br />
      <span v-for="addr in iface.addresses" :key="addr">{{ addr }}<br /></span>
    </div>
  </div>
</template>
