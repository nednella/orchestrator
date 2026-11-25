import { useState } from 'react'
import { tipAlign, type Bar } from '../statsChart'

type StatsBarsProps = { bars: Bar[]; label: string; caption: string; tip(value: number): string; axis(value: number): string }

const TIP_ALIGN = {
  start: 'left-0',
  center: 'left-1/2 -translate-x-1/2',
  end: 'right-0',
}

// Bars lose their gap as they get thin, so every one keeps some width.
const gapFor = (count: number) => (count <= 31 ? 'gap-1' : count <= 60 ? 'gap-0.5' : 'gap-px')

export function StatsBars({ bars, label, caption, tip, axis }: StatsBarsProps) {
  const [hovered, setHovered] = useState<number | null>(null)
  const top = Math.max(0, ...bars.map((b) => b.value))
  const gap = gapFor(bars.length)
  // Few bars reach their widest and sit in the middle of their column, so their ticks do too.
  const few = bars.length <= 14
  return (
    <figure className="relative pt-10">
      <div role="group" aria-label={label} className={`relative flex h-40 items-end border-b border-line-strong pr-8 ${gap}`} onMouseLeave={() => setHovered(null)}>
        {top > 0 &&
          [1, 0.5].map((at) => (
            <span key={at} aria-hidden="true" className="pointer-events-none absolute inset-x-0 border-t border-dashed border-line" style={{ bottom: `${at * 100}%` }}>
              <span className="absolute top-0 right-0 -translate-y-1/2 bg-(--bg-card) pl-1 text-label text-soft tabular-nums">{axis(at === 1 ? top : top / 2)}</span>
            </span>
          ))}
        {bars.map((bar, i) => {
          const text = `${bar.label}: ${tip(bar.value)}`
          const on = hovered === i
          const current = i === bars.length - 1
          return (
            <div key={bar.day} role="img" aria-label={text} className="relative flex h-full min-w-0 flex-1 items-end" onMouseEnter={() => setHovered(i)}>
              <span
                className={`mx-auto block w-full max-w-10 rounded-t-[3px] transition-colors duration-100 ${on ? 'bg-(--chart-bar-on)' : current ? 'bg-[repeating-linear-gradient(135deg,var(--chart-bar)_0_2px,transparent_2px_5px)] shadow-[inset_0_0_0_1px_var(--chart-bar)]' : 'bg-(--chart-bar)'}`}
                style={{ height: bar.value > 0 && top > 0 ? `${Math.max((bar.value / top) * 100, 2)}%` : 0 }}
              />
              {on && (
                <span
                  aria-hidden="true"
                  className={`absolute bottom-[calc(100%+0.25rem)] z-(--z-popup) rounded-md bg-(--bg-glass) px-2 py-1 text-small whitespace-nowrap shadow-[0_0_0_1px_color-mix(in_srgb,var(--text)_10%,transparent),var(--shadow-tip)] backdrop-blur-[16px] backdrop-saturate-[1.08] ${TIP_ALIGN[tipAlign(i, bars.length)]}`}
                >
                  {text}
                </span>
              )}
            </div>
          )
        })}
      </div>
      <div aria-hidden="true" className={`mt-1 flex pr-8 ${gap}`}>
        {bars.map((bar) => (
          <span key={bar.day} className={`min-w-0 flex-1 overflow-visible text-label whitespace-nowrap text-soft ${few ? 'text-center' : ''}`}>
            {bar.tick}
          </span>
        ))}
      </div>
      <figcaption className="mt-2 text-small text-soft">{caption}</figcaption>
    </figure>
  )
}
