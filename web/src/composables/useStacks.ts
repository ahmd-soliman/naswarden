import type { Container } from './usePoolSocket'

export type StackHealth = 'green' | 'yellow' | 'red' | 'gray'
export type MemberTone = 'ok' | 'crit' | 'gray'

export interface Stack {
  name: string
  composeFile: string
  icon: string
  members: Container[]
  running: number
  total: number // members that count toward health (clean stops excluded)
  cpu: number
  mem: number
  health: StackHealth
}

export const isStopped = (c: Container) => c.state === 'exited' || c.state === 'created' || c.state === 'dead'

// A container that exited with code 0 stopped cleanly -- a one-shot helper
// (nextcloud-aio-watchtower) or a stack someone stopped on purpose. That is
// not a fault, so it must not turn its stack yellow.
export const isCleanStop = (c: Container) => c.state === 'exited' && c.exit_code === 0

export function memberTone(c: Container): MemberTone {
  if (c.state === 'running') return 'ok'
  if (isCleanStop(c) || c.state === 'created' || c.state === 'paused') return 'gray'
  return 'crit' // restarting, dead, or exited with a non-zero code
}

export function memberBadge(c: Container): { cls: string; label: string } {
  const tone = memberTone(c)
  const cls = tone === 'ok' ? 'badge--green' : tone === 'crit' ? 'badge--red' : 'badge--gray'
  const label = c.state === 'exited' && c.exit_code !== 0 ? `exited (${c.exit_code})` : c.state
  return { cls, label }
}

const HEALTH_RANK: Record<StackHealth, number> = { red: 0, yellow: 1, green: 2, gray: 3 }

// Group containers by compose project. Problems sort first, then A-Z, so the
// stack that needs attention is the first thing on the page.
export function buildStacks(containers: Container[]): Stack[] {
  const groups = new Map<string, Container[]>()
  for (const c of containers) {
    if (!c.stack) continue // loose containers only appear on the Containers tab
    const list = groups.get(c.stack)
    if (list) list.push(c)
    else groups.set(c.stack, [c])
  }

  const stacks = [...groups].map(([name, all]): Stack => {
    const members = [...all].sort(
      (a, b) => Number(b.state === 'running') - Number(a.state === 'running') || a.name.localeCompare(b.name),
    )
    const counted = members.filter((m) => !isCleanStop(m))
    const up = counted.filter((m) => m.state === 'running')
    const live = members.filter((m) => m.state === 'running')
    const health: StackHealth =
      counted.length === 0 ? 'gray' : up.length === counted.length ? 'green' : up.length === 0 ? 'red' : 'yellow'
    return {
      name,
      composeFile: members.find((m) => m.compose_file)?.compose_file ?? '',
      icon: members.find((m) => m.icon)?.icon ?? '',
      members,
      running: up.length,
      total: counted.length,
      cpu: live.reduce((sum, m) => sum + m.cpu_percent, 0),
      mem: live.reduce((sum, m) => sum + m.mem_used, 0),
      health,
    }
  })
  return stacks.sort((a, b) => HEALTH_RANK[a.health] - HEALTH_RANK[b.health] || a.name.localeCompare(b.name))
}

export const stackBadgeLabel = (s: Stack) => (s.health === 'gray' ? 'stopped' : `${s.running}/${s.total} running`)

// ---- icons -------------------------------------------------------------
// App icons come from the Dashboard Icons set (homarr-labs/dashboard-icons),
// resolved at view time by slug. Names below are the stacks whose compose
// project name differs from the icon's slug -- each verified to resolve.
// `naswarden.icon` on any service overrides everything.
const ICON_ALIASES: Record<string, string> = {
  drawio: 'draw-io',
  'wiki-js': 'wikijs',
  'nextcloud-aio': 'nextcloud',
  'portainer-agent': 'portainer',
  'gitlab-cloud-runner': 'gitlab',
  'gitlab-incus-runner': 'gitlab',
  'gitlab-runner-incus-gitlab-runner': 'gitlab',
  filebeat: 'elastic',
  'node-exporter': 'prometheus',
  'open-speed-test': 'openspeedtest',
  lxconsole: 'incus',
}

// "ghcr.io/jellyfin/jellyfin:12.1" -> "jellyfin"
const imageSlug = (image: string) => image.split('@')[0].split(':')[0].split('/').pop() ?? ''

export function iconCandidates(s: Stack): string[] {
  const list = [s.icon, ICON_ALIASES[s.name], s.name, imageSlug(s.members[0]?.image ?? '')]
  return [...new Set(list.filter((x): x is string => !!x))]
}
