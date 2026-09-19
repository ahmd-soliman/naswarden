<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps<{ open: boolean; title: string }>()
const emit = defineEmits<{ close: [] }>()

const drawer = ref<HTMLElement | null>(null)
const closeButton = ref<HTMLButtonElement | null>(null)
// Where focus was when the drawer opened -- put back on close so keyboard
// users land on the card they activated, not at the top of the page.
let returnFocusTo: HTMLElement | null = null

watch(
  () => props.open,
  async (open) => {
    if (open) {
      returnFocusTo = document.activeElement as HTMLElement | null
      await nextTick()
      closeButton.value?.focus()
    } else if (returnFocusTo && document.contains(returnFocusTo)) {
      returnFocusTo.focus()
      returnFocusTo = null
    }
  },
)

const FOCUSABLE = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'

// Escape closes; Tab is trapped inside the drawer while it is open (it is
// aria-modal, so the page behind must not be reachable).
function onKeydown(e: KeyboardEvent) {
  if (!props.open) return
  if (e.key === 'Escape') {
    emit('close')
    return
  }
  if (e.key !== 'Tab' || !drawer.value) return
  const items = [...drawer.value.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => !el.hasAttribute('disabled'))
  if (items.length === 0) return
  const first = items[0]
  const last = items[items.length - 1]
  if (e.shiftKey && document.activeElement === first) {
    e.preventDefault()
    last.focus()
  } else if (!e.shiftKey && document.activeElement === last) {
    e.preventDefault()
    first.focus()
  } else if (!drawer.value.contains(document.activeElement)) {
    e.preventDefault()
    first.focus()
  }
}

onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="drawer-backdrop" :class="{ open }" aria-hidden="true" @click="emit('close')" />
  <div
    ref="drawer"
    class="drawer"
    :class="{ open }"
    role="dialog"
    aria-modal="true"
    aria-labelledby="drawer-title"
    :inert="!open"
  >
    <div class="drawer__header">
      <span id="drawer-title" class="drawer__title">{{ title }}</span>
      <button ref="closeButton" class="drawer__close" aria-label="Close" @click="emit('close')">&times;</button>
    </div>
    <div class="drawer__body">
      <slot />
    </div>
  </div>
</template>
