import { onBeforeUnmount, onMounted, ref } from 'vue'

// True while the viewport is at least `minWidth` px wide. Used to dock the
// detail panel beside the content on desktop instead of overlaying it.
export function useWide(minWidth = 1200) {
  const wide = ref(false)
  let mql: MediaQueryList | undefined
  const update = () => (wide.value = !!mql?.matches)
  onMounted(() => {
    mql = window.matchMedia(`(min-width: ${minWidth}px)`)
    update()
    mql.addEventListener('change', update)
  })
  onBeforeUnmount(() => mql?.removeEventListener('change', update))
  return wide
}
