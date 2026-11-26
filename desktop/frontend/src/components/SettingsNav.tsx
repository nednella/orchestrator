import { SETTINGS_NAV } from '../settingsSections'
import type { SettingsSectionId } from '../settingsSections'

type SettingsNavProps = { section: SettingsSectionId; onSelect(id: SettingsSectionId): void }

// Below the md width the nav is an icon rail: the labels and group headings go.
export function SettingsNav({ section, onSelect }: SettingsNavProps) {
  return (
    <nav aria-label="Settings sections" className="flex w-14 flex-none flex-col gap-4 overflow-y-auto border-r border-line p-2 md:w-60">
      {SETTINGS_NAV.map((group, i) => (
        <div key={group.heading ?? i} className="flex flex-col gap-0.5">
          {group.heading && <h2 className="px-2 pb-1 text-small font-medium text-dim max-md:hidden">{group.heading}</h2>}
          {group.sections.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              className={`flex h-8 items-center gap-2.5 rounded-md px-2.5 text-left text-body max-md:justify-center max-md:px-0 ${id === section ? 'bg-(--bg-row-active) font-medium text-ink' : 'text-soft hover:bg-(--bg-row-hover)'}`}
              aria-current={id === section ? 'page' : undefined}
              title={label}
              onClick={() => onSelect(id)}
            >
              <Icon size={16} aria-hidden="true" className={id === section ? 'text-ink' : 'text-(--sidebar-icon)'} />
              <span className="truncate max-md:hidden">{label}</span>
            </button>
          ))}
        </div>
      ))}
    </nav>
  )
}
