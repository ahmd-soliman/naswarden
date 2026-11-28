import { describe, expect, it } from 'vitest'
import type { ProtectionTask } from './usePoolSocket'
import {
  ago,
  groupByKind,
  lastRunText,
  matchesProtection,
  protectionAttention,
  protectionSummary,
  retentionText,
  scheduleText,
  startsCollapsed,
  taskBadge,
} from './protection'

let n = 0
const task = (o: Partial<ProtectionTask> = {}): ProtectionTask => ({
  kind: 'cloudsync',
  id: ++n,
  name: 'Backup',
  source: '/mnt/tank/a',
  exclude: [],
  enabled: true,
  state: 'OK',
  overdue: false,
  ...o,
})

describe('taskBadge', () => {
  it('has one badge per state, and overdue replaces OK', () => {
    expect(taskBadge(task())).toEqual({ label: 'OK', tone: 'green' })
    expect(taskBadge(task({ state: 'OK', overdue: true }))).toEqual({ label: 'Overdue', tone: 'yellow' })
    expect(taskBadge(task({ state: 'RUNNING', progress_percent: 41.6 }))).toEqual({ label: 'Running 42%', tone: 'blue' })
    expect(taskBadge(task({ state: 'RUNNING' })).label).toBe('Running')
    expect(taskBadge(task({ state: 'FAILED' }))).toEqual({ label: 'Failed', tone: 'red' })
    expect(taskBadge(task({ state: 'NEVER' }))).toEqual({ label: 'Never ran', tone: 'gray' })
    expect(taskBadge(task({ state: 'DISABLED' }))).toEqual({ label: 'Disabled', tone: 'gray' })
    expect(taskBadge(task({ state: 'UNKNOWN' })).tone).toBe('gray')
  })
})

describe('attention and summary', () => {
  const tasks = [task(), task({ state: 'FAILED' }), task({ overdue: true }), task({ state: 'DISABLED', overdue: true })]
  it('counts failed and overdue, not disabled or never', () => {
    expect(protectionAttention(tasks)).toEqual({ failed: 1, overdue: 1, count: 2, level: 'crit' })
    expect(protectionAttention([task({ overdue: true })]).level).toBe('warn')
    expect(protectionAttention([task({ state: 'NEVER' }), task({ state: 'RUNNING' })]).count).toBe(0)
    expect(protectionAttention([]).count).toBe(0)
  })
  it('summarises the problems, or the size when there are none', () => {
    expect(protectionSummary(tasks)).toEqual({ text: '1 failed · 1 overdue', tone: 'crit' })
    expect(protectionSummary([task({ overdue: true })])).toEqual({ text: '1 overdue', tone: 'warn' })
    expect(protectionSummary([task(), task()])).toEqual({ text: '2 tasks, none failed', tone: 'ok' })
    expect(protectionSummary([task()]).text).toBe('1 task, none failed')
  })
})

describe('groupByKind', () => {
  it('keeps only kinds that exist, in a fixed order, problems first', () => {
    const g = groupByKind([
      task({ kind: 'rsync', name: 'r' }),
      task({ kind: 'snapshot', name: 'b' }),
      task({ kind: 'snapshot', name: 'a' }),
      task({ kind: 'snapshot', name: 'z', state: 'FAILED' }),
      task({ kind: 'snapshot', name: 'off', state: 'DISABLED' }),
      task({ kind: 'snapshot', name: 'late', overdue: true }),
    ])
    expect(g.map((x) => x.kind)).toEqual(['snapshot', 'rsync'])
    expect(g[0].tasks.map((t) => t.name)).toEqual(['z', 'late', 'a', 'b', 'off'])
    expect(groupByKind([])).toEqual([])
  })
  it('folds only a long list that is all fine', () => {
    const ok = [task(), task(), task(), task(), task(), task(), task()]
    expect(startsCollapsed(ok)).toBe(true)
    expect(startsCollapsed(ok.slice(0, 6))).toBe(false)
    expect(startsCollapsed([...ok, task({ state: 'FAILED' })])).toBe(false)
    expect(startsCollapsed([...ok, task({ overdue: true })])).toBe(false)
    expect(startsCollapsed([...ok, task({ state: 'RUNNING' })])).toBe(false)
  })
})

describe('times', () => {
  it('says how long ago', () => {
    expect(ago(1000, 1030)).toBe('just now')
    expect(ago(1000, 1000 + 5 * 60)).toBe('5 min ago')
    expect(ago(1000, 1000 + 6 * 3600 + 5)).toBe('6 h ago')
    expect(ago(1000, 1000 + 3 * 86400)).toBe('3 d ago')
    expect(ago(2000, 1000)).toBe('just now') // clock skew never goes negative
  })
  it('writes the last run line', () => {
    const now = 100000
    expect(lastRunText(task({ time_finished: now - 6 * 3600 }), now)).toBe('last run 6 h ago')
    expect(lastRunText(task({ kind: 'snapshot', time_finished: now - 600 }), now)).toBe('last snapshot 10 min ago')
    expect(lastRunText(task({ state: 'RUNNING', time_started: now - 120 }), now)).toBe('started 2 min ago')
    expect(lastRunText(task({ state: 'NEVER' }), now)).toBe('never ran')
    expect(lastRunText(task({ state: 'DISABLED' }), now)).toBe('disabled, never ran')
    expect(lastRunText(task({ state: 'FAILED', time_started: now - 60 * 5 }), now)).toBe('last run 5 min ago')
  })
})

describe('scheduleText', () => {
  it('words the common cron shapes and leaves the rest as cron', () => {
    expect(scheduleText('0 2 * * *')).toBe('daily at 02:00')
    expect(scheduleText('30 3 * * mon,thu')).toBe('Monday, Thursday at 03:30')
    expect(scheduleText('0 3 * * 1-5')).toBe('0 3 * * 1-5')
    expect(scheduleText('0 3 * * 7')).toBe('Sunday at 03:00')
    expect(scheduleText('*/15 * * * *')).toBe('every 15 min')
    expect(scheduleText('0 * * * *', '08:00-18:00')).toBe('hourly at :00 (08:00–18:00)')
    expect(scheduleText('0 */6 * * *')).toBe('every 6 h at :00')
    expect(scheduleText('0 4 1 * *')).toBe('0 4 1 * *')
    expect(scheduleText('')).toBe('')
    expect(scheduleText(undefined)).toBe('')
  })
  it('words retention', () => {
    expect(retentionText('2 WEEK')).toBe('2 weeks')
    expect(retentionText('1 MONTH')).toBe('1 month')
    expect(retentionText('last 7')).toBe('last 7')
    expect(retentionText(undefined)).toBe('')
  })
})

describe('search', () => {
  it('matches name, path, remote and kind', () => {
    const t = task({ name: 'Photos', source: '/mnt/tank/p', remote: 'S3 bucket', kind: 'cloudsync' })
    for (const q of ['photos', 'tank/p', 'bucket', 'cloud sync', '']) expect(matchesProtection(t, q)).toBe(true)
    expect(matchesProtection(t, 'rsync')).toBe(false)
  })
})
