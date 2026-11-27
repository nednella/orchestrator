import type { ReactNode } from 'react'
import type { ListOption } from '../listOptions'
import { ListMenu } from './ListMenu'
import { ListMenuItem } from './ListMenuItem'

type ListRadioMenuProps = {
  label: string
  title?: string
  trigger: ReactNode
  options: ListOption[]
  value: string
  onChange(value: string): void
}

export function ListRadioMenu({ label, title, trigger, options, value, onChange }: ListRadioMenuProps) {
  return (
    <ListMenu label={label} title={title} trigger={trigger}>
      {(close) =>
        options.map((o) => (
          <ListMenuItem
            key={o.value}
            role="menuitemradio"
            checked={o.value === value}
            icon={o.icon}
            disabled={o.unavailable !== undefined}
            title={o.unavailable}
            onClick={() => {
              close()
              onChange(o.value)
            }}
          >
            {o.label}
          </ListMenuItem>
        ))
      }
    </ListMenu>
  )
}
