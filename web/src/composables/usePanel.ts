import { getCurrentInstance, nextTick, onBeforeUnmount, onMounted, type Ref } from 'vue'

// Cards and table rows whose place on screen is kept while the panel docks.
const ANCHORS = '.app__content .card--clickable, .app__content .lt__row'
const OPEN_ANCHORS = '.app__content .card--open, .app__content .lt__row--open'

// Opening and closing the detail panel. target is what the panel shows (null:
// closed); wide is true when the panel docks beside the content instead of
// covering it.
export function usePanel<T>(target: Ref<T | null>, wide: Ref<boolean>) {
  // Clicking the card or row whose detail is already open closes the panel.
  function toggle(t: T, open: boolean) {
    select(open ? null : t)
  }

  // Opening or closing the docked panel narrows or widens the content, so the
  // card grids reflow and cards would jump between rows. A view transition
  // moves each card from its old place to its new one instead (style.css).
  function select(t: T | null) {
    const docking = wide.value && (target.value === null) !== (t === null)
    if (!docking) {
      target.value = t
      return
    }
    if (!document.startViewTransition || matchMedia('(prefers-reduced-motion: reduce)').matches) {
      void dock(t)
      return
    }
    // The panel slides with the cards, on the transition's own clock; its CSS
    // transition would run on its own and briefly overlap the widening cards.
    document.documentElement.classList.add('docking')
    const vt = document.startViewTransition(() => dock(t))
    void vt.finished.finally(() => document.documentElement.classList.remove('docking'))
  }

  // The reflow also changes the page height (fewer columns, more rows), which
  // would carry the card the user clicked out of view: keep it where it was.
  async function dock(t: T | null) {
    const anchor = dockAnchor()
    const before = anchor?.getBoundingClientRect().top
    target.value = t
    await nextTick()
    if (anchor?.isConnected && before !== undefined) {
      window.scrollBy({ top: anchor.getBoundingClientRect().top - before, behavior: 'instant' })
    }
  }

  // The card or row just pressed, else the focused one (keyboard), else the
  // one whose panel is open (closing with the panel's own button or Escape).
  let lastPressed: Element | null = null
  function dockAnchor(): Element | null {
    const pressed = lastPressed?.isConnected ? lastPressed : null
    lastPressed = null
    return pressed?.closest(ANCHORS) ?? document.activeElement?.closest(ANCHORS) ?? document.querySelector(OPEN_ANCHORS)
  }

  // A clicked table row does not take focus, so remember what was pressed.
  function onPress(e: Pick<PointerEvent, 'target'>) {
    lastPressed = e.target instanceof Element ? e.target : null
  }
  if (getCurrentInstance()) {
    onMounted(() => document.addEventListener('pointerdown', onPress, true))
    onBeforeUnmount(() => document.removeEventListener('pointerdown', onPress, true))
  }

  return { toggle, select, onPress }
}
