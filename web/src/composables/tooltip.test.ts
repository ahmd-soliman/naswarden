import { describe, expect, it } from 'vitest'
import { clippedTitle } from './tooltip'

describe('clippedTitle', () => {
  it('returns the full text when the cell is clipped', () => {
    const el = { scrollWidth: 300, clientWidth: 200, textContent: '  ghcr.io/nextcloud-releases/all-in-one:latest \n' }
    expect(clippedTitle(el)).toBe('ghcr.io/nextcloud-releases/all-in-one:latest')
  })
  it('collapses whitespace from multi-part cells', () => {
    const el = { scrollWidth: 300, clientWidth: 200, textContent: 'ONLINE\n   mirror-0' }
    expect(clippedTitle(el)).toBe('ONLINE mirror-0')
  })
  it('returns nothing when the text fits, so no redundant tooltip appears', () => {
    expect(clippedTitle({ scrollWidth: 200, clientWidth: 200, textContent: 'sda' })).toBe('')
  })
  it('handles an empty cell', () => {
    expect(clippedTitle({ scrollWidth: 10, clientWidth: 5, textContent: null })).toBe('')
  })
})
