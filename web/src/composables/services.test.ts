import { describe, expect, it } from 'vitest'
import type { Pool, Service, Share } from './usePoolSocket'
import { serviceLabel, serviceStateText, serviceTone, shareIssue, sharesOf } from './services'

const svc = (o: Partial<Service>): Service => ({ name: 'cifs', state: 'RUNNING', enable: true, problem: false, ...o })
const share = (o: Partial<Share>): Share => ({ service: 'cifs', name: 's', path: '/mnt/tank/s', enabled: true, read_only: false, ...o })
const pool = (name: string, status = 'ONLINE') => ({ name, status }) as Pool

describe('services', () => {
  it('labels known services and passes unknown ones through', () => {
    expect(serviceLabel('cifs')).toBe('SMB')
    expect(serviceLabel('iscsitarget')).toBe('iSCSI')
    expect(serviceLabel('somethingnew')).toBe('somethingnew')
  })
  it('tones: running ok, enabled-but-stopped is the problem', () => {
    expect(serviceTone(svc({}))).toBe('ok')
    expect(serviceTone(svc({ state: 'STOPPED', problem: true }))).toBe('crit')
    expect(serviceTone(svc({ state: 'STOPPED', enable: false }))).toBe('gray')
    expect(serviceStateText(svc({ state: 'STOPPED', problem: true }))).toMatch(/start on boot/)
    expect(serviceStateText(svc({ enable: false }))).toMatch(/not set to start/)
  })
  it('picks the shares of one service', () => {
    const all = [share({ service: 'cifs' }), share({ service: 'nfs' })]
    expect(sharesOf(all, 'nfs')).toHaveLength(1)
    expect(sharesOf(all, 'ssh')).toHaveLength(0)
  })
})

describe('shareIssue', () => {
  const pools = [pool('tank'), pool('old', 'OFFLINE'), pool('worn', 'DEGRADED')]
  it('is fine on an online or degraded pool', () => {
    expect(shareIssue(share({}), pools)).toBe('')
    expect(shareIssue(share({ path: '/mnt/worn/x' }), pools)).toBe('')
  })
  it('flags a missing path', () => {
    expect(shareIssue(share({ path: '' }), pools)).toBe('No path set')
  })
  it('flags a pool that is gone or offline', () => {
    expect(shareIssue(share({ path: '/mnt/gone/x' }), pools)).toMatch(/gone is not available/)
    expect(shareIssue(share({ path: '/mnt/old/x' }), pools)).toMatch(/old is offline/)
  })
  it('does not judge a disabled share, or a path outside /mnt', () => {
    expect(shareIssue(share({ path: '', enabled: false }), pools)).toBe('')
    expect(shareIssue(share({ path: '/var/data' }), pools)).toBe('')
  })
})
