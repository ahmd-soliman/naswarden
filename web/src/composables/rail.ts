// In "All" view the side rail hints which section is being read as the page
// scrolls. At the very top of the page there is nothing to hint yet: the
// section under the trigger line is arbitrary (Pools on a laptop, Disks on a
// wide screen, where Server and Pools share the first row), so only "All" is
// highlighted until the user actually scrolls.
export const SCROLLED_PX = 24

export function isScrolledDown(scrollY: number): boolean {
  return scrollY > SCROLLED_PX
}

export function isScrollHint(active: string, scrolledDown: boolean, spied: string, section: string): boolean {
  return active === 'all' && scrolledDown && spied === section
}
