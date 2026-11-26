import { describe, expect, it } from 'vitest'
import { poolCapacity } from './poolCapacity'

describe('poolCapacity', () => {
  it('uses the usable figures when the backend sent them', () => {
    // 4x12 TB RAIDZ1: the zpool reports ~43.6 TB raw, only ~32 TB holds files.
    const c = poolCapacity({ size: 43.6e12, allocated: 2.9e12, usable_size: 32e12, usable_used: 1.9e12 })
    expect(c).toEqual({ used: 1.9e12, total: 32e12, percent: 6, usable: true })
  })
  it('falls back to the raw figures without a root dataset, never 0', () => {
    expect(poolCapacity({ size: 1000, allocated: 250 })).toEqual({ used: 250, total: 1000, percent: 25, usable: false })
    expect(poolCapacity({ size: 1000, allocated: 250, usable_size: 0, usable_used: 0 }).usable).toBe(false)
  })
  it('does not divide by zero', () => {
    expect(poolCapacity({ size: 0, allocated: 0 }).percent).toBe(0)
  })
  it('a full usable pool is 100 and an empty one 0', () => {
    expect(poolCapacity({ size: 10, allocated: 1, usable_size: 100, usable_used: 100 }).percent).toBe(100)
    expect(poolCapacity({ size: 10, allocated: 1, usable_size: 100, usable_used: 0 }).percent).toBe(0)
  })
})
