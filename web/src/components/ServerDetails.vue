<script setup lang="ts">
import { computed } from 'vue'
import type { Interface, ServerInfo } from '../composables/usePoolSocket'
import { formatBitrate } from '../composables/format'

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

// Link utilisation for a physical interface: the busier direction against the
// link speed. Bridges report the speed of a virtual link, so no bar for them.
function utilisation(iface: Interface): number | null {
  if (iface.type !== 'PHYSICAL' || !iface.speed_mbps || iface.rx_kbps === undefined || iface.tx_kbps === undefined) return null
  return (Math.max(iface.rx_kbps, iface.tx_kbps) / (iface.speed_mbps * 1000)) * 100
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
    <div v-for="iface in server.interfaces" :key="iface.name" class="iface">
      <div class="iface__head">
        <strong>{{ iface.name }}</strong>
        <span class="badge badge--green">up &middot; {{ iface.speed || iface.type.toLowerCase() }}</span>
      </div>
      <div v-if="iface.rx_kbps !== undefined && iface.tx_kbps !== undefined" class="iface__rates">
        <div><small>Received</small>{{ formatBitrate(iface.rx_kbps) }}</div>
        <div><small>Sent</small>{{ formatBitrate(iface.tx_kbps) }}</div>
      </div>
      <template v-if="utilisation(iface) !== null">
        <div
          class="iface__util"
          role="meter"
          :aria-label="`${iface.name} link utilisation`"
          aria-valuemin="0"
          aria-valuemax="100"
          :aria-valuenow="Math.round(utilisation(iface)!)"
        >
          <i :style="{ width: Math.max(utilisation(iface)!, 0.5) + '%' }" />
        </div>
        <div class="iface__addr">{{ utilisation(iface)!.toFixed(1) }}% of link speed</div>
      </template>
      <div v-for="addr in iface.addresses" :key="addr" class="iface__addr">{{ addr }}</div>
    </div>
  </div>
</template>

<style scoped>
.iface {
  padding: 0.7rem 0;
  border-bottom: 1px solid var(--border);
}
.iface:last-child {
  border-bottom: 0;
}
.iface__head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.45rem;
}
.iface__rates {
  display: flex;
  gap: 1.6rem;
  font-variant-numeric: tabular-nums;
}
.iface__rates small {
  display: block;
  color: var(--text-dim);
  font-size: 0.8rem;
  text-transform: uppercase;
}
.iface__util {
  height: 6px;
  background: var(--border);
  border-radius: 999px;
  margin: 0.5rem 0 0.35rem;
  overflow: hidden;
}
.iface__util i {
  display: block;
  height: 100%;
  background: var(--stack);
}
.iface__addr {
  color: var(--text-dim);
  font-family: ui-monospace, monospace;
  font-size: 0.85rem;
}
</style>
