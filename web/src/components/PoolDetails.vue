<script setup lang="ts">
import type { Pool } from '../composables/usePoolSocket'

defineProps<{ pool: Pool }>()
</script>

<template>
  <div class="drawer__section">
    <h3>Layout</h3>
    <div class="drawer__kv"><span>Fragmentation</span><span>{{ pool.fragmentation }}%</span></div>
    <div v-for="vdev in pool.vdevs" :key="vdev.name" class="drawer__kv">
      <span>{{ vdev.name }}</span><span>{{ vdev.type }} ({{ vdev.children.length }} disks)</span>
    </div>
  </div>

  <div class="drawer__section">
    <h3>Disk members</h3>
    <div v-if="pool.vdevs.length === 0" class="drawer__empty">No disk info available.</div>
    <template v-for="vdev in pool.vdevs" :key="vdev.name">
      <div v-for="disk in vdev.children" :key="disk.disk" class="drawer__kv">
        <span>{{ disk.disk }}</span>
        <span>
          {{ disk.status }}
          <template v-if="disk.read_errors + disk.write_errors + disk.checksum_errors > 0">
            ({{ disk.read_errors + disk.write_errors + disk.checksum_errors }} errors)
          </template>
        </span>
      </div>
    </template>
  </div>
</template>
