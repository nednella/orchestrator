import { Bookmark, Check, ChevronDown, Plus } from 'lucide-react'
import { useState } from 'react'
import { viewNameError } from '../listViews'
import { ListMenu } from './ListMenu'
import { ListMenuItem } from './ListMenuItem'
import { ViewNameDialog } from './ViewNameDialog'

type ViewName = { id: string; name: string }

export type ViewsMenuProps = {
  kind: 'issue' | 'PR'
  views: ViewName[]
  active: ViewName | null
  modified: boolean
  // The name the Save dialog starts with.
  suggested: string
  onApply(id: string): void
  onSave(name: string): void
  onUpdate(): void
  onRename(name: string): void
  onDelete(): void
}

export function ViewsMenu({ kind, views, active, modified, suggested, onApply, onSave, onUpdate, onRename, onDelete }: ViewsMenuProps) {
  const [dialog, setDialog] = useState<'save' | 'rename' | null>(null)
  const placeholder = kind === 'issue' ? 'Issues Assigned to Me' : 'PRs Requesting My Review'
  const own = dialog === 'rename' ? (active?.id ?? '') : ''

  return (
    <>
      <ListMenu
        label={`Saved ${kind} views`}
        title={active?.name ?? `Saved ${kind} views`}
        trigger={
          <>
            <Bookmark size={16} className="text-soft" />
            <span className="max-w-40 truncate">{active?.name ?? 'Views'}</span>
            {modified && <span aria-label="Modified view">*</span>}
            <ChevronDown size={12} className="text-soft" />
          </>
        }
      >
        {(close) => (
          <>
            {views.map((v) => (
              <ListMenuItem
                key={v.id}
                icon={v.id === active?.id && !modified ? <Check size={14} /> : <Bookmark size={14} />}
                title={v.name}
                onClick={() => {
                  close()
                  onApply(v.id)
                }}
              >
                {v.name}
              </ListMenuItem>
            ))}
            {views.length > 0 && <div role="separator" className="mx-2 my-1 h-px bg-line" />}
            <ListMenuItem
              icon={<Plus size={14} />}
              onClick={() => {
                close()
                setDialog('save')
              }}
            >
              Save current view…
            </ListMenuItem>
            {active && modified && (
              <ListMenuItem
                onClick={() => {
                  close()
                  onUpdate()
                }}
              >
                Update saved view
              </ListMenuItem>
            )}
            {active && (
              <>
                <ListMenuItem
                  onClick={() => {
                    close()
                    setDialog('rename')
                  }}
                >
                  Rename view…
                </ListMenuItem>
                <ListMenuItem
                  danger
                  onClick={() => {
                    close()
                    onDelete()
                  }}
                >
                  Delete view
                </ListMenuItem>
              </>
            )}
          </>
        )}
      </ListMenu>
      {dialog && (
        <ViewNameDialog
          title={`${dialog === 'save' ? 'Save' : 'Rename'} ${kind} view`}
          placeholder={placeholder}
          initial={dialog === 'save' ? suggested : (active?.name ?? '')}
          error={(name) => viewNameError(name, views, own)}
          onClose={() => setDialog(null)}
          onSave={(name) => {
            setDialog(null)
            if (dialog === 'save') onSave(name)
            else onRename(name)
          }}
        />
      )}
    </>
  )
}
