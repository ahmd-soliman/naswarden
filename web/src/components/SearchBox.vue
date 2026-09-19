<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

const props = defineProps<{ modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const input = ref<HTMLInputElement | null>(null)

function clear() {
  emit('update:modelValue', '')
  input.value?.focus()
}

// Escape clears the query first; a second press leaves the field.
function onEscape() {
  if (props.modelValue) emit('update:modelValue', '')
  else input.value?.blur()
}

// "/" jumps to search from anywhere -- unless the user is already typing in a
// field or a drawer is open (the drawer is modal, the page behind is inert).
function onGlobalKey(e: KeyboardEvent) {
  if (e.key !== '/' || e.ctrlKey || e.metaKey || e.altKey) return
  const target = e.target as HTMLElement | null
  if (target && (target.closest('input, textarea, select, [contenteditable="true"]') || target.isContentEditable)) return
  if (document.querySelector('.drawer.open')) return
  e.preventDefault()
  input.value?.focus()
  input.value?.select()
}

onMounted(() => document.addEventListener('keydown', onGlobalKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onGlobalKey))
</script>

<template>
  <div class="search" role="search">
    <svg class="search__icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
      <circle cx="11" cy="11" r="7" />
      <path d="M21 21l-4.3-4.3" />
    </svg>
    <input
      ref="input"
      class="search__input"
      type="text"
      :value="modelValue"
      aria-label="Search stacks, containers, pools and datasets"
      placeholder="Search…"
      autocomplete="off"
      spellcheck="false"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      @keydown.esc.prevent="onEscape"
    />
    <button v-if="modelValue" class="search__clear" type="button" aria-label="Clear search" @click="clear">
      &times;
    </button>
    <kbd v-else class="search__hint" aria-hidden="true">/</kbd>
  </div>
</template>

<style scoped>
.search {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  max-width: 420px;
  min-width: 0;
}

.search__icon {
  position: absolute;
  left: 0.7rem;
  width: 16px;
  height: 16px;
  color: var(--text-dim);
  pointer-events: none;
}

.search__input {
  width: 100%;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 0.5rem 2.4rem 0.5rem 2.15rem;
  color: var(--text);
  font: inherit;
  font-size: 0.9rem;
}

.search__input::placeholder {
  color: var(--text-dim);
}

.search__input:focus-visible {
  outline: 2px solid var(--ok-t);
  outline-offset: 1px;
}

.search__clear,
.search__hint {
  position: absolute;
  right: 0.45rem;
}

.search__clear {
  width: 28px;
  height: 28px;
  background: none;
  border: none;
  border-radius: 6px;
  color: var(--text-dim);
  font-size: 1.2rem;
  line-height: 1;
  cursor: pointer;
}

.search__clear:hover {
  color: var(--text);
}

.search__hint {
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 0 0.4rem;
  color: var(--text-dim);
  font: 0.8rem ui-monospace, monospace;
  pointer-events: none;
}
</style>
