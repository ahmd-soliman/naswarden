import { describe, expect, it } from 'vitest'
import { versionDetail, versionLabel } from './useVersion'

describe('version display', () => {
  it('prefixes release numbers with v', () => {
    expect(versionLabel('1.0.0')).toBe('v1.0.0')
    expect(versionLabel('1.2.3-rc1')).toBe('v1.2.3-rc1')
  })
  it('shows non-release builds as they are', () => {
    expect(versionLabel('main-4c827ea')).toBe('main-4c827ea')
    expect(versionLabel('dev')).toBe('dev')
  })
  it('details the short commit and build time when known', () => {
    expect(versionDetail({ version: '1.0.0', commit: '4c827ea57e152828d8a3', date: '2026-09-19T12:00:00Z' })).toBe(
      'commit 4c827ea · built 2026-09-19T12:00:00Z',
    )
  })
  it('omits what an unstamped build does not know', () => {
    expect(versionDetail({ version: 'dev', commit: 'unknown', date: 'unknown' })).toBe('')
    expect(versionDetail({ version: 'x', commit: 'abcdef123', date: 'unknown' })).toBe('commit abcdef1')
  })
})
