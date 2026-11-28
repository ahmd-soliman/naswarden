import type { ProtectionTask } from './usePoolSocket'

export type Kind = ProtectionTask['kind']

// Display order and names of the task kinds.
export const KINDS: { kind: Kind; label: string }[] = [
  { kind: 'snapshot', label: 'Periodic snapshots' },
  { kind: 'cloudsync', label: 'Cloud Sync' },
  { kind: 'rsync', label: 'Rsync' },
  { kind: 'cloud_backup', label: 'TrueCloud backup' },
]
export const kindLabel = (k: Kind) => KINDS.find((x) => x.kind === k)?.label ?? k

export const taskKey = (t: ProtectionTask) => `${t.kind}:${t.id}`

export type Tone = 'green' | 'yellow' | 'red' | 'blue' | 'gray'

// One badge per task. Overdue is a run that did not happen, so it replaces OK:
// "OK" next to a missing backup would be the wrong thing to read.
export function taskBadge(t: ProtectionTask): { label: string; tone: Tone } {
  if (t.state === 'OK' && t.overdue) return { label: 'Overdue', tone: 'yellow' }
  switch (t.state) {
    case 'OK':
      return { label: 'OK', tone: 'green' }
    case 'RUNNING':
      return { label: t.progress_percent !== undefined ? `Running ${Math.round(t.progress_percent)}%` : 'Running', tone: 'blue' }
    case 'FAILED':
      return { label: 'Failed', tone: 'red' }
    case 'NEVER':
      return { label: 'Never ran', tone: 'gray' }
    case 'DISABLED':
      return { label: 'Disabled', tone: 'gray' }
    default:
      return { label: 'Unknown', tone: 'gray' }
  }
}

const hasProblem = (t: ProtectionTask) => t.state === 'FAILED' || (t.state === 'OK' && t.overdue)

// Failed or overdue tasks: what the section header and the rail badge count.
export function protectionAttention(tasks: ProtectionTask[]): { failed: number; overdue: number; count: number; level: 'crit' | 'warn' } {
  const failed = tasks.filter((t) => t.state === 'FAILED').length
  const overdue = tasks.filter((t) => t.state === 'OK' && t.overdue).length
  return { failed, overdue, count: failed + overdue, level: failed > 0 ? 'crit' : 'warn' }
}

// The section header's count: the problems if there are any, else how many.
export function protectionSummary(tasks: ProtectionTask[]): { text: string; tone: 'crit' | 'warn' | 'ok' } {
  const a = protectionAttention(tasks)
  if (a.count > 0) {
    const parts = [a.failed > 0 ? `${a.failed} failed` : '', a.overdue > 0 ? `${a.overdue} overdue` : ''].filter(Boolean)
    return { text: parts.join(' · '), tone: a.failed > 0 ? 'crit' : 'warn' }
  }
  const n = tasks.length
  return { text: `${n} ${n === 1 ? 'task' : 'tasks'}, none failed`, tone: 'ok' }
}

const ORDER: Record<ProtectionTask['state'], number> = { FAILED: 0, RUNNING: 2, NEVER: 3, UNKNOWN: 3, OK: 4, DISABLED: 5 }
const rank = (t: ProtectionTask) => (hasProblem(t) && t.state === 'OK' ? 1 : ORDER[t.state])

// Tasks of each kind that exists, problems first, then by name.
export function groupByKind(tasks: ProtectionTask[]): { kind: Kind; label: string; tasks: ProtectionTask[] }[] {
  return KINDS.map(({ kind, label }) => ({
    kind,
    label,
    tasks: tasks.filter((t) => t.kind === kind).sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name)),
  })).filter((g) => g.tasks.length > 0)
}

// A long list that is all fine starts folded, so it does not push the tasks
// that need a look off the screen. Anything else starts open.
export function startsCollapsed(tasks: ProtectionTask[]): boolean {
  return tasks.length > 6 && tasks.every((t) => (t.state === 'OK' && !t.overdue) || t.state === 'DISABLED')
}

export function ago(ts: number, nowSec: number): string {
  const s = Math.max(0, Math.floor(nowSec - ts))
  if (s < 60) return 'just now'
  if (s < 3600) return `${Math.floor(s / 60)} min ago`
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`
  return `${Math.floor(s / 86400)} d ago`
}

// "last run 6 h ago" for the row's second line.
export function lastRunText(t: ProtectionTask, nowSec: number): string {
  if (t.state === 'RUNNING') return t.time_started ? `started ${ago(t.time_started, nowSec)}` : 'running now'
  const at = t.time_finished ?? t.time_started
  if (at === undefined) return t.state === 'DISABLED' ? 'disabled, never ran' : 'never ran'
  return `${t.kind === 'snapshot' ? 'last snapshot' : 'last run'} ${ago(at, nowSec)}`
}

const DOW = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
const DOW_SHORT = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat']
const pad = (n: string) => n.padStart(2, '0')

// The common cron shapes in words; anything else is shown as the cron fields.
export function scheduleText(cron: string | undefined, window?: string): string {
  if (!cron) return ''
  const f = cron.split(' ')
  if (f.length !== 5) return cron
  const [m, h, dom, mon, dow] = f
  const num = /^\d+$/
  const win = window ? ` (${window.replace('-', '–')})` : ''
  const everyN = (s: string) => /^\*\/(\d+)$/.exec(s)?.[1]
  if (dom === '*' && mon === '*') {
    if (num.test(m) && h === '*' && dow === '*') return `hourly at :${pad(m)}${win}`
    if (everyN(m) && h === '*' && dow === '*') return `every ${everyN(m)} min${win}`
    if (num.test(m) && everyN(h) && dow === '*') return `every ${everyN(h)} h at :${pad(m)}${win}`
    if (num.test(m) && num.test(h)) {
      const at = `${pad(h)}:${pad(m)}`
      if (dow === '*') return `daily at ${at}`
      const days = dow
        .split(',')
        .map((d) => (num.test(d) ? DOW[Number(d) % 7] : DOW[DOW_SHORT.indexOf(d.toLowerCase())]))
      if (days.every(Boolean)) return `${days.join(', ')} at ${at}`
    }
  }
  return cron
}

// Snapshot retention "2 WEEK" -> "2 weeks"; backups say "last 7".
export function retentionText(r: string | undefined): string {
  const m = r ? /^(\d+) (HOUR|DAY|WEEK|MONTH|YEAR)$/.exec(r) : null
  if (!m) return r ?? ''
  const unit = m[2].toLowerCase()
  return `${m[1]} ${unit}${m[1] === '1' ? '' : 's'}`
}

export function durationText(sec: number): string {
  if (sec < 60) return `${sec} s`
  const mins = Math.floor(sec / 60)
  if (mins < 60) return `${mins} min ${sec % 60} s`
  return `${Math.floor(mins / 60)} h ${mins % 60} min`
}

export function matchesProtection(t: ProtectionTask, q: string): boolean {
  if (!q) return true
  return [t.name, t.source, t.remote ?? '', kindLabel(t.kind), t.kind].some((x) => x.toLowerCase().includes(q))
}
