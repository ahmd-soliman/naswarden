import { onMounted, ref } from 'vue'

export interface BuildInfo {
  version: string
  commit: string
  date: string
}

// "1.0.0" -> "v1.0.0"; anything else ("main-4c827ea", "dev") is shown as is.
export function versionLabel(version: string): string {
  return /^\d+\.\d+\.\d+/.test(version) ? `v${version}` : version
}

// Hover text: the commit and build time, when the build was stamped with them.
export function versionDetail(info: BuildInfo): string {
  const parts: string[] = []
  if (info.commit && info.commit !== 'unknown') parts.push(`commit ${info.commit.slice(0, 7)}`)
  if (info.date && info.date !== 'unknown') parts.push(`built ${info.date}`)
  return parts.join(' · ')
}

// Which build is serving this page. Fetched once; the footer just stays hidden
// if the endpoint is missing (an older backend) or unreachable.
export function useVersion() {
  const info = ref<BuildInfo | null>(null)
  onMounted(async () => {
    try {
      const res = await fetch('/version')
      if (res.ok) info.value = (await res.json()) as BuildInfo
    } catch {
      // leave hidden
    }
  })
  return info
}
