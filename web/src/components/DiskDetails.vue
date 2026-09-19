<script setup lang="ts">
import { computed } from 'vue'
import type { Disk } from '../composables/usePoolSocket'
import { TEMP_CRIT, TEMP_WARN, diskBadge, diskTypeLabel, tempTone } from '../composables/useDisks'
import { formatBytes, formatCapacity, formatRate } from '../composables/format'

const props = defineProps<{ disk: Disk }>()

const badge = computed(() => diskBadge(props.disk))
const isMember = computed(() => props.disk.status !== '')
const tempClass = computed(() => (props.disk.temp_c === undefined ? '' : `t-${tempTone(props.disk.temp_c)}`))
const errClass = (n: number) => (n > 0 ? 't-warn' : 't-ok')
</script>

<template>
  <div class="drawer__section">
    <h3>Overview</h3>
    <div class="drawer__kv"><span>Status</span><span class="badge" :class="badge.cls">{{ badge.label }}</span></div>
    <div class="drawer__kv"><span>Model</span><span>{{ disk.model || '--' }}</span></div>
    <div class="drawer__kv"><span>Serial</span><span>{{ disk.serial || '--' }}</span></div>
    <div class="drawer__kv"><span>Size</span><span>{{ formatCapacity(disk.size) }}</span></div>
    <div class="drawer__kv"><span>Type</span><span>{{ diskTypeLabel(disk) }}{{ disk.bus ? ` · ${disk.bus}` : '' }}</span></div>
    <div class="drawer__kv">
      <span>Pool</span>
      <span>{{ disk.pool || 'none' }}<template v-if="disk.vdev"> ({{ disk.vdev }})</template></span>
    </div>
  </div>

  <div class="drawer__section">
    <h3>Temperature</h3>
    <div class="drawer__kv">
      <span>Now</span>
      <span :class="tempClass">{{ disk.temp_c !== undefined ? `${Math.round(disk.temp_c)} °C` : disk.standby ? 'standby' : '--' }}</span>
    </div>
    <div v-if="disk.temp_7d_min !== undefined" class="drawer__kv">
      <span>Last 7 days</span>
      <span>min {{ Math.round(disk.temp_7d_min) }} · avg {{ disk.temp_7d_avg?.toFixed(1) }} · max {{ Math.round(disk.temp_7d_max ?? 0) }} °C</span>
    </div>
    <div class="drawer__kv"><span>Alert thresholds</span><span>{{ TEMP_WARN }} °C warn · {{ TEMP_CRIT }} °C critical</span></div>
  </div>

  <div v-if="disk.read_bytes !== undefined" class="drawer__section">
    <h3>Activity</h3>
    <div class="drawer__kv"><span>Read now</span><span>{{ disk.read_bytes_per_sec !== undefined ? formatRate(disk.read_bytes_per_sec) : '--' }}</span></div>
    <div class="drawer__kv"><span>Write now</span><span>{{ disk.write_bytes_per_sec !== undefined ? formatRate(disk.write_bytes_per_sec) : '--' }}</span></div>
    <div class="drawer__kv"><span>Read since pool import</span><span>{{ formatBytes(disk.read_bytes) }}</span></div>
    <div class="drawer__kv"><span>Written since pool import</span><span>{{ formatBytes(disk.write_bytes ?? 0) }}</span></div>
  </div>

  <div v-if="isMember" class="drawer__section">
    <h3>ZFS errors</h3>
    <div class="drawer__kv"><span>Read</span><span :class="errClass(disk.read_errors)">{{ disk.read_errors }}</span></div>
    <div class="drawer__kv"><span>Write</span><span :class="errClass(disk.write_errors)">{{ disk.write_errors }}</span></div>
    <div class="drawer__kv"><span>Checksum</span><span :class="errClass(disk.checksum_errors)">{{ disk.checksum_errors }}</span></div>
  </div>
</template>

<style scoped>
.t-ok {
  color: var(--ok-t);
}
.t-warn {
  color: var(--warn-t);
}
.t-crit {
  color: var(--crit-t);
}
</style>
