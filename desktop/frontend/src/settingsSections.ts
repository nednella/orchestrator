import { Bot, GitBranch, Globe, Info, Palette, Settings2 } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export type SettingsSectionId = 'general' | 'appearance' | 'agents' | 'browser' | 'git' | 'about'

export type SettingsSectionEntry = { id: SettingsSectionId; label: string; icon: LucideIcon }

export type SettingsNavGroup = { heading?: string; sections: SettingsSectionEntry[] }

export const SETTINGS_NAV: SettingsNavGroup[] = [
  {
    sections: [
      { id: 'general', label: 'General', icon: Settings2 },
      { id: 'appearance', label: 'Appearance', icon: Palette },
    ],
  },
  {
    heading: 'Agents',
    sections: [
      { id: 'agents', label: 'Agents', icon: Bot },
      { id: 'browser', label: 'Browser', icon: Globe },
    ],
  },
  { heading: 'Code', sections: [{ id: 'git', label: 'Git and pull requests', icon: GitBranch }] },
  { sections: [{ id: 'about', label: 'About', icon: Info }] },
]

export function sectionLabel(id: SettingsSectionId): string {
  for (const group of SETTINGS_NAV) {
    const found = group.sections.find((s) => s.id === id)
    if (found) return found.label
  }
  return ''
}
