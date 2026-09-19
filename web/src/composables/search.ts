import type { Container, Dataset, Disk, Pool, VM } from './usePoolSocket'
import type { Stack } from './useStacks'

// Case-insensitive substring match; an empty query matches everything.
export const normalizeQuery = (raw: string) => raw.trim().toLowerCase()
const has = (text: string, q: string) => text.toLowerCase().includes(q)

export const matchesPool = (p: Pool, q: string) => !q || has(p.name, q)
export const matchesDataset = (d: Dataset, q: string) => !q || has(d.name, q)

// A stack matches on its own name OR any member's name -- searching "mongo"
// should surface the komodo stack it runs in, not just the container.
export const matchesStack = (s: Stack, q: string) => !q || has(s.name, q) || s.members.some((m) => has(m.name, q))

// A container also matches on its stack and image ("jellyfin" finds the
// container even if it is named "media-server").
export const matchesContainer = (c: Container, q: string) =>
  !q || has(c.name, q) || has(c.stack, q) || has(c.image, q)

// A VM or Incus instance matches on its name, OS, manager (TrueNAS vs Incus), type (KVM vs LXC), passthrough devices, or IP.
export const matchesVM = (v: VM, q: string) =>
  !q ||
  has(v.name, q) ||
  has(v.os, q) ||
  has(v.manager || '', q) ||
  has(v.type, q) ||
  has(v.is_vm ? 'kvm' : 'lxc', q) ||
  (v.passthrough && v.passthrough.some((p) => has(p, q))) ||
  v.ipv4.some((ip) => has(ip, q))

// A disk matches on its device name, model, serial, pool or type (HDD/SSD).
export const matchesDisk = (d: Disk, q: string) =>
  !q || has(d.name, q) || has(d.model, q) || has(d.serial, q) || has(d.pool, q) || has(d.type, q)
