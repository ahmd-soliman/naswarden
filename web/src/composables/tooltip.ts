// Table cells clip long text with an ellipsis. Give a clipped cell its full
// text as a tooltip -- and only then, so cells that fit stay tooltip-free.
export function clippedTitle(el: { scrollWidth: number; clientWidth: number; textContent: string | null }): string {
  return el.scrollWidth > el.clientWidth ? (el.textContent ?? '').replace(/\s+/g, ' ').trim() : ''
}
