import { describe, expect, it } from 'vitest'
import { appsUpdateText, safeLink, updateBadge } from './updateInfo'

const base = { available: false, apps_with_updates: 0, checked_at: 1 }

describe('updateBadge', () => {
  it('shows nothing without an update', () => {
    expect(updateBadge(undefined)).toBeNull()
    expect(updateBadge(base)).toBeNull()
  })
  it('names the version when known', () => {
    expect(updateBadge({ ...base, available: true, version: '25.10.8' })).toBe('Update 25.10.8')
    expect(updateBadge({ ...base, available: true })).toBe('Update available')
  })
})

describe('appsUpdateText', () => {
  it('counts apps and hides at zero', () => {
    expect(appsUpdateText(undefined)).toBeNull()
    expect(appsUpdateText(base)).toBeNull()
    expect(appsUpdateText({ ...base, apps_with_updates: 1 })).toBe('1 app has an update')
    expect(appsUpdateText({ ...base, apps_with_updates: 3 })).toBe('3 apps have updates')
  })
})

describe('safeLink', () => {
  it('accepts only http(s)', () => {
    expect(safeLink('https://truenas.com/notes')).toBe('https://truenas.com/notes')
    expect(safeLink('javascript:alert(1)')).toBeNull()
    expect(safeLink(undefined)).toBeNull()
  })
})
