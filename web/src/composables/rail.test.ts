import { describe, expect, it } from 'vitest'
import { isScrollHint, isScrolledDown } from './rail'

describe('rail scroll hint', () => {
  it('counts as scrolled only past a few pixels, so a fresh page or a bounce at the top is not', () => {
    expect(isScrolledDown(0)).toBe(false)
    expect(isScrolledDown(24)).toBe(false)
    expect(isScrolledDown(25)).toBe(true)
  })
  it('shows no hint at the top of the page, whatever section is under the trigger line', () => {
    for (const s of ['server', 'pools', 'disks', 'datasets']) expect(isScrollHint('all', false, 'disks', s)).toBe(false)
  })
  it('hints the section in view once scrolled, in All view only', () => {
    expect(isScrollHint('all', true, 'disks', 'disks')).toBe(true)
    expect(isScrollHint('all', true, 'disks', 'pools')).toBe(false)
    expect(isScrollHint('disks', true, 'disks', 'disks')).toBe(false) // a chosen section is "active", not a hint
  })
})
