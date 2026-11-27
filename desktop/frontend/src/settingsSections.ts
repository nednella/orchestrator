import { Globe, HardDrive, Info, FolderTree, MessagesSquare, Palette, Wrench } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export type SettingsSectionId = 'appearance' | 'threads' | 'browser' | 'worktrees' | 'data' | 'maintenance' | 'about'

export type SettingsSectionEntry = { id: SettingsSectionId; label: string; icon: LucideIcon }

export type SettingsNavGroup = { heading?: string; sections: SettingsSectionEntry[] }

export const SETTINGS_NAV: SettingsNavGroup[] = [
  { heading: 'General', sections: [{ id: 'appearance', label: 'Appearance', icon: Palette }] },
  {
    heading: 'Agents',
    sections: [
      { id: 'threads', label: 'Threads', icon: MessagesSquare },
      { id: 'browser', label: 'Browser', icon: Globe },
    ],
  },
  { heading: 'Code', sections: [{ id: 'worktrees', label: 'Worktrees', icon: FolderTree }] },
  { heading: 'Storage', sections: [{ id: 'data', label: 'Data', icon: HardDrive }] },
  {
    sections: [
      { id: 'maintenance', label: 'Maintenance', icon: Wrench },
      { id: 'about', label: 'About', icon: Info },
    ],
  },
]

export function sectionLabel(id: SettingsSectionId): string {
  for (const group of SETTINGS_NAV) {
    const found = group.sections.find((s) => s.id === id)
    if (found) return found.label
  }
  return ''
}
