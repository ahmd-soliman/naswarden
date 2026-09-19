<script setup lang="ts">
import { computed } from 'vue'
import type { Disk } from '../composables/usePoolSocket'
import { tempRangeLabel, tempTone } from '../composables/useDisks'

const props = defineProps<{ disk: Disk }>()

// Fixed scale so bars are comparable between rows.
const LO = 20
const HI = 70
const pos = (v: number) => `${Math.max(0, Math.min(100, ((v - LO) / (HI - LO)) * 100))}%`

const label = computed(() => tempRangeLabel(props.disk))
const tone = computed(() => (props.disk.temp_c === undefined ? 'gray' : tempTone(props.disk.temp_c)))
const hasRange = computed(() => props.disk.temp_7d_min !== undefined && props.disk.temp_7d_max !== undefined)
</script>

<template>
  <span class="temp" role="img" :aria-label="label" :title="label">
    <template v-if="disk.temp_c !== undefined">
      <span class="temp__now" :class="`temp__now--${tone}`" aria-hidden="true">{{ Math.round(disk.temp_c) }}&deg;</span>
      <template v-if="hasRange">
        <span class="temp__bar" aria-hidden="true">
          <i :style="{ left: pos(disk.temp_7d_min!), width: `calc(${pos(disk.temp_7d_max!)} - ${pos(disk.temp_7d_min!)})` }" />
          <u :class="`temp__mark--${tone}`" :style="{ left: `calc(${pos(disk.temp_c)} - 1px)` }" />
        </span>
        <span class="temp__range" aria-hidden="true">{{ Math.round(disk.temp_7d_min!) }}&ndash;{{ Math.round(disk.temp_7d_max!) }}&deg;</span>
      </template>
    </template>
    <span v-else class="temp__none" aria-hidden="true">&mdash; {{ disk.standby ? 'standby' : '' }}</span>
  </span>
</template>

<style scoped>
.temp {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}
.temp__now {
  min-width: 2.6em;
  text-align: right;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.temp__now--ok {
  color: var(--ok-t);
}
.temp__now--warn {
  color: var(--warn-t);
}
.temp__now--crit {
  color: var(--crit-t);
}
.temp__bar {
  position: relative;
  width: 56px;
  height: 6px;
  background: var(--border);
  border-radius: 999px;
}
.temp__bar i {
  position: absolute;
  top: 0;
  bottom: 0;
  background: #5b6472;
  border-radius: 999px;
}
.temp__bar u {
  position: absolute;
  top: -2px;
  width: 3px;
  height: 10px;
  border-radius: 2px;
  text-decoration: none;
}
.temp__mark--ok {
  background: var(--ok-t);
}
.temp__mark--warn {
  background: var(--warn-t);
}
.temp__mark--crit {
  background: var(--crit-t);
}
.temp__range,
.temp__none {
  color: var(--text-dim);
  font-variant-numeric: tabular-nums;
}
</style>
