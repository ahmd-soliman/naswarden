<script setup lang="ts">
import type { Dataset } from '../composables/usePoolSocket'

defineProps<{ dataset: Dataset }>()

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
  <div class="drawer__section">
    <h3>Properties</h3>
    <div class="drawer__kv"><span>Mountpoint</span><span>{{ dataset.mountpoint }}</span></div>
    <div class="drawer__kv"><span>Compression</span><span>{{ dataset.compression }} ({{ dataset.compress_ratio }})</span></div>
    <div class="drawer__kv"><span>Record size</span><span>{{ dataset.recordsize }}</span></div>
    <div class="drawer__kv"><span>Encryption</span><span>{{ dataset.encrypted ? 'on' : 'off' }}</span></div>
  </div>

  <div class="drawer__section">
    <h3>Snapshots</h3>
    <div class="drawer__kv"><span>Space used by snapshots</span><span>{{ formatBytes(dataset.used_by_snapshots) }}</span></div>
  </div>
</template>
