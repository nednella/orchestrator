import { ListFilter } from 'lucide-react'
import { useState } from 'react'
import type { FilterRow } from '../listOptions'
import { FilterRowItem } from './FilterRowItem'
import { ListMenu } from './ListMenu'

type FiltersMenuProps = { rows: FilterRow[]; count: number }

export function FiltersMenu({ rows, count }: FiltersMenuProps) {
  const [openRow, setOpenRow] = useState('')
  return (
    <ListMenu
      align="end"
      label="Filters"
      trigger={
        <>
          <ListFilter size={16} className="text-soft" />
          Filters
          {count > 0 && <span className="rounded-full bg-hover px-1.5 text-label text-soft tabular-nums">{count}</span>}
        </>
      }
    >
      {(_close, menuId) =>
        rows.map((row) =>
          row.kind === 'separator' ? (
            <div key={row.id} role="separator" className="mx-2 my-1 h-px bg-line" />
          ) : (
            <FilterRowItem key={row.id} row={row} menuId={menuId} open={openRow === row.id} onOpen={(open) => setOpenRow(open ? row.id : openRow === row.id ? '' : openRow)} />
          ),
        )
      }
    </ListMenu>
  )
}
