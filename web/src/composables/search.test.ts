import { describe, expect, it } from 'vitest'
import type { Container, VM } from './usePoolSocket'
import { matchesContainer, matchesPool, matchesStack, matchesVM, normalizeQuery } from './search'
import { buildStacks } from './useStacks'

const cont = (o: Partial<Container>) => ({ name: 'n', image: 'i', stack: '', state: 'running', ...o }) as Container

describe('search', () => {
  it('normalizes case and whitespace; empty matches everything', () => {
    expect(normalizeQuery('  JeLLy ')).toBe('jelly')
    expect(matchesPool({ name: 'tank' } as never, '')).toBe(true)
  })
  it('matches names case-insensitively as substrings', () => {
    expect(matchesPool({ name: 'Tank' } as never, 'an')).toBe(true)
    expect(matchesPool({ name: 'Tank' } as never, 'zz')).toBe(false)
  })
  it('a stack matches through its members', () => {
    const [s] = buildStacks([cont({ name: 'mongo', stack: 'komodo' }), cont({ name: 'core', stack: 'komodo' })])
    expect(matchesStack(s, 'mongo')).toBe(true)
    expect(matchesStack(s, 'komodo')).toBe(true)
    expect(matchesStack(s, 'nope')).toBe(false)
  })
  it('a container matches on stack and image too', () => {
    const x = cont({ name: 'media-server', stack: 'media', image: 'jellyfin/jellyfin' })
    expect(matchesContainer(x, 'jellyfin')).toBe(true)
    expect(matchesContainer(x, 'media')).toBe(true)
    expect(matchesContainer(x, 'plex')).toBe(false)
  })
  it('a VM matches on kind, manager, passthrough and IP', () => {
    const v = { name: 'win', os: 'Windows 10', type: 'virtual-machine', manager: 'truenas', is_vm: true, ipv4: ['10.0.0.5'], passthrough: ['GTX 1050 Ti'] } as VM
    for (const q of ['win', 'windows', 'kvm', 'truenas', 'gtx', '10.0.0']) expect(matchesVM(v, q)).toBe(true)
    expect(matchesVM(v, 'lxc')).toBe(false)
  })
})
