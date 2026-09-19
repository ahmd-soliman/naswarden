<script setup lang="ts" generic="T">
import { computed, ref } from 'vue'
import { clippedTitle } from '../composables/tooltip'

export interface Column<R> {
  key: string
  label: string
  // Sort value; also the default cell text when no #cell-<key> slot is given.
  value: (row: R) => string | number
  align?: 'right'
}

const props = defineProps<{
  rows: T[]
  columns: Column<T>[]
  rowKey: (row: T) => string
  isActive?: (row: T) => boolean
  caption: string
  defaultSort?: { key: string; dir: 'asc' | 'desc' }
}>()
const emit = defineEmits<{ select: [row: T] }>()

const sort = ref<{ key: string; dir: 'asc' | 'desc' } | null>(props.defaultSort ?? null)

function toggleSort(key: string) {
  const s = sort.value
  sort.value = s && s.key === key ? { key, dir: s.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: 'asc' }
}

const sorted = computed(() => {
  const s = sort.value
  const col = s && props.columns.find((c) => c.key === s.key)
  if (!s || !col) return props.rows
  const sign = s.dir === 'asc' ? 1 : -1
  return [...props.rows].sort((a, b) => {
    const x = col.value(a)
    const y = col.value(b)
    const cmp =
      typeof x === 'number' && typeof y === 'number'
        ? x - y
        : String(x).localeCompare(String(y), undefined, { numeric: true, sensitivity: 'base' })
    return cmp * sign
  })
})

// Set on hover, when the cell's real width is known.
function showFullText(e: MouseEvent) {
  const el = e.currentTarget as HTMLElement
  const full = clippedTitle(el)
  if (full) el.title = full
  else el.removeAttribute('title')
}

const ariaSort = (key: string) =>
  sort.value?.key === key ? (sort.value.dir === 'asc' ? 'ascending' : 'descending') : 'none'
</script>

<template>
  <div class="lt">
    <table>
      <caption class="sr-only">{{ caption }}</caption>
      <thead>
        <tr>
          <th v-for="col in columns" :key="col.key" scope="col" :aria-sort="ariaSort(col.key)" :class="{ 'lt__num': col.align === 'right' }">
            <button type="button" class="lt__sort" @click="toggleSort(col.key)">
              {{ col.label }}
              <span class="lt__arrow" aria-hidden="true">{{ sort?.key === col.key ? (sort.dir === 'asc' ? '▲' : '▼') : '' }}</span>
            </button>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in sorted"
          :key="rowKey(row)"
          class="lt__row"
          :class="{ 'lt__row--open': isActive?.(row) }"
          @click="emit('select', row)"
        >
          <td v-for="(col, i) in columns" :key="col.key" :class="{ 'lt__num': col.align === 'right' }" @mouseenter="showFullText">
            <button v-if="i === 0" type="button" class="lt__open" @click.stop="emit('select', row)">
              <slot :name="`cell-${col.key}`" :row="row">{{ col.value(row) }}</slot>
            </button>
            <slot v-else :name="`cell-${col.key}`" :row="row">{{ col.value(row) }}</slot>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.lt {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow-x: auto;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}
thead th {
  text-align: left;
  font-weight: 600;
  color: var(--text-dim);
  font-size: 0.8rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
  padding: 0;
}
.lt__sort {
  all: unset;
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.55rem 0.75rem;
  cursor: pointer;
  width: 100%;
}
.lt__sort:hover {
  color: var(--text);
}
.lt__sort:focus-visible,
.lt__open:focus-visible {
  outline: 2px solid var(--ok-t);
  outline-offset: -2px;
}
.lt__arrow {
  font-size: 0.65rem;
  min-width: 0.7em;
}
tbody td {
  padding: 0.4rem 0.75rem;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
  max-width: 28ch;
  overflow: hidden;
  text-overflow: ellipsis;
}
tbody tr:last-child td {
  border-bottom: 0;
}
.lt__row {
  cursor: pointer;
}
.lt__row:hover {
  background: rgba(255, 255, 255, 0.03);
}
.lt__row--open {
  /* kept subtle: status badges on it must stay >= 4.5:1 */
  background: rgba(255, 255, 255, 0.04);
  box-shadow: inset 3px 0 0 var(--accent, var(--text-dim));
}
.lt__num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
thead .lt__num .lt__sort {
  justify-content: flex-end;
}
.lt__open {
  all: unset;
  box-sizing: border-box;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 600;
  color: var(--text);
  max-width: 100%;
}
</style>
