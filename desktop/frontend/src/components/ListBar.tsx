import { ArrowDownUp, Plug2, RefreshCw } from 'lucide-react'
import type { FilterRow, ListOption, SortConfig } from '../listOptions'
import { LIST_BUTTON } from '../listStyle'
import { FiltersMenu } from './FiltersMenu'
import { ListRadioMenu } from './ListRadioMenu'
import { ListSearch } from './ListSearch'
import type { ListSearchProps } from './ListSearch'
import { ViewsMenu } from './ViewsMenu'
import type { ViewsMenuProps } from './ViewsMenu'

export type ListBarProps = {
  noun: 'issues' | 'pull requests'
  search: ListSearchProps
  sort: SortConfig
  filters: { rows: FilterRow[]; count: number }
  views: ViewsMenuProps
  providers: ListOption[]
  host: string
  onHost(host: string): void
  loading: boolean
  onRefresh(): void
}

// ListBar is the action bar over the issue list and the pull request list: search, sort, filters, saved views, the
// provider and refresh. The search takes a line of its own where the list is narrow, and the buttons wrap under it.
export function ListBar({ noun, search, sort, filters, views, providers, host, onHost, loading, onRefresh }: ListBarProps) {
  const chosen = providers.find((p) => p.value === host)
  const options: ListOption[] = [{ value: '', label: 'All', icon: <Plug2 size={14} /> }, ...providers]
  return (
    <div className="@container flex-none px-2 pt-2 pb-1">
      <div className="flex flex-wrap items-center gap-2">
        <ListSearch {...search} />
        <ListRadioMenu
          label={`Sort ${noun}: ${sort.options.find((o) => o.value === sort.value)?.label ?? ''}`}
          trigger={
            <>
              <ArrowDownUp size={16} className="text-soft" />
              Sort
            </>
          }
          options={sort.options}
          value={sort.value}
          onChange={sort.onChange}
        />
        <FiltersMenu {...filters} />
        <ViewsMenu {...views} />
        <ListRadioMenu
          label={chosen ? `Filter by provider: ${chosen.label}` : 'Filter by provider'}
          title={chosen ? `Filter by provider: ${chosen.label}` : undefined}
          trigger={
            chosen ? (
              chosen.icon
            ) : (
              <>
                <Plug2 size={16} className="text-soft" />
                All
              </>
            )
          }
          options={options}
          value={host}
          onChange={onHost}
        />
        <button className={`${LIST_BUTTON} w-8 justify-center px-0`} title={`Refresh ${noun}`} aria-label={`Refresh ${noun}`} disabled={loading} onClick={onRefresh}>
          <RefreshCw size={16} className={`text-soft ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>
    </div>
  )
}
