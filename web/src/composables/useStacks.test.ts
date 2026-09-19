import { describe, expect, it } from 'vitest'
import type { Container } from './usePoolSocket'
import { buildStacks, containerIconCandidates, iconCandidates, isCleanStop, memberBadge, stackBadgeLabel } from './useStacks'

const c = (o: Partial<Container>): Container => ({
  name: 'x', image: 'img', state: 'running', status: '', cpu_percent: 0, mem_used: 0, mem_limit: 0,
  started_at: '', restart_policy: '', command: '', mounts: [], networks: [], ports: [],
  stack: 's', compose_file: '', icon: '', exit_code: 0, ...o,
})

describe('isCleanStop', () => {
  it('treats exit 0, 143 and 137 as intentional', () => {
    for (const code of [0, 143, 137]) expect(isCleanStop(c({ state: 'exited', exit_code: code }))).toBe(true)
  })
  it('treats a crash or an OOM kill as not clean', () => {
    expect(isCleanStop(c({ state: 'exited', exit_code: 1 }))).toBe(false)
    expect(isCleanStop(c({ state: 'exited', exit_code: 137, oom_killed: true }))).toBe(false)
  })
  it('never calls a running container a clean stop, and honours optional', () => {
    expect(isCleanStop(c({ state: 'running' }))).toBe(false)
    expect(isCleanStop(c({ state: 'exited', exit_code: 1, optional: true }))).toBe(true)
  })
})

describe('buildStacks', () => {
  it('skips loose containers and groups by project', () => {
    const stacks = buildStacks([c({ name: 'a', stack: '' }), c({ name: 'b', stack: 'p' }), c({ name: 'c', stack: 'p' })])
    expect(stacks.map((s) => s.name)).toEqual(['p'])
    expect(stacks[0].members).toHaveLength(2)
  })
  it('computes health from counted members, excluding clean stops', () => {
    const [s] = buildStacks([
      c({ name: 'a', state: 'running' }),
      c({ name: 'b', state: 'exited', exit_code: 0 }),
    ])
    expect(s.health).toBe('green')
    expect(s.total).toBe(1)
    expect(stackBadgeLabel(s)).toBe('1/1 running')
  })
  it('is yellow when some crashed, red when all did, gray when all stopped cleanly', () => {
    const mix = [c({ name: 'a' }), c({ name: 'b', state: 'exited', exit_code: 1 })]
    expect(buildStacks(mix)[0].health).toBe('yellow')
    expect(buildStacks([mix[1]])[0].health).toBe('red')
    const stopped = buildStacks([c({ state: 'exited', exit_code: 0 })])[0]
    expect(stopped.health).toBe('gray')
    expect(stackBadgeLabel(stopped)).toBe('stopped')
  })
  it('sorts problems first, then A-Z', () => {
    const names = buildStacks([
      c({ name: 'a1', stack: 'alpha' }),
      c({ name: 'b1', stack: 'bravo', state: 'exited', exit_code: 1 }),
    ]).map((s) => s.name)
    expect(names).toEqual(['bravo', 'alpha'])
  })
  it('sums cpu and memory of running members only', () => {
    const [s] = buildStacks([
      c({ name: 'a', cpu_percent: 2, mem_used: 10 }),
      c({ name: 'b', state: 'exited', exit_code: 0, cpu_percent: 9, mem_used: 90 }),
    ])
    expect(s.cpu).toBe(2)
    expect(s.mem).toBe(10)
  })
})

describe('memberBadge', () => {
  it('labels stops and crashes', () => {
    expect(memberBadge(c({ state: 'exited', exit_code: 0 })).label).toBe('stopped')
    expect(memberBadge(c({ state: 'exited', exit_code: 143 })).label).toBe('stopped (143)')
    expect(memberBadge(c({ state: 'exited', exit_code: 2 })).label).toBe('exited (2)')
    expect(memberBadge(c({ oom_killed: true, state: 'exited', exit_code: 137 })).label).toBe('oom killed')
  })
})

describe('icon candidates', () => {
  it('prefers the label override, then alias, name, image slug, deduplicated', () => {
    const stack = buildStacks([c({ stack: 'nextcloud-aio', image: 'ghcr.io/nextcloud-releases/all-in-one:latest', icon: 'custom' })])[0]
    expect(iconCandidates(stack)).toEqual(['custom', 'nextcloud', 'nextcloud-aio', 'all-in-one'])
    expect(containerIconCandidates(c({ name: 'jellyfin', image: 'jellyfin/jellyfin:10' }))).toEqual(['jellyfin'])
  })
})
