import type { VM } from './usePoolSocket'

export function vmIconCandidates(vm: VM): string[] {
  const list: string[] = []
  const nameLower = vm.name.toLowerCase()
  const osLower = (vm.os || '').toLowerCase()

  // 0. Explicit user configuration override (e.g. incus config set <name> user.icon <slug>)
  if (vm.config?.['naswarden.icon']) list.push(vm.config['naswarden.icon'])
  if (vm.config?.['user.icon']) list.push(vm.config['user.icon'])

  // 1. Name-based application / service slugs
  if (nameLower.startsWith('k8s') || nameLower.includes('kubernetes')) {
    list.push('kubernetes', 'k8s')
  } else if (nameLower.includes('openstack')) {
    list.push('openstack')
  } else if (nameLower.includes('gitlab')) {
    list.push('gitlab', 'gitlab-runner')
  } else if (nameLower.includes('db') || nameLower.includes('postgres')) {
    list.push('postgresql', 'postgres')
  } else if (nameLower.includes('claude')) {
    list.push('claude', 'anthropic')
  }

  // 2. OS-based slugs
  if (osLower.includes('win')) {
    list.push('windows-11', 'windows-10', 'windows')
  } else if (osLower.includes('ubuntu')) {
    list.push('ubuntu')
  } else if (osLower.includes('debian')) {
    list.push('debian')
  } else if (osLower.includes('fedora')) {
    list.push('fedora')
  } else if (osLower.includes('arch')) {
    list.push('arch-linux', 'arch')
  } else if (osLower.includes('alpine')) {
    list.push('alpine-linux', 'alpine')
  }

  // 3. Fallback to VM name
  list.push(nameLower)

  // 4. Platform fallbacks
  if (vm.manager === 'truenas') {
    list.push('truenas', 'qemu')
  } else if (vm.is_vm) {
    list.push('incus', 'qemu', 'linux')
  } else {
    list.push('incus', 'lxc', 'linux')
  }

  return [...new Set(list.filter(Boolean))]
}
