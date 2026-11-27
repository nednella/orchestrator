import type { ThemeName } from '../types'

type WireframePaneProps = { theme: ThemeName; clip?: 'left' | 'right' }

const CLIPS = {
  left: 'polygon(0 0, calc(50% - 1px) 0, calc(50% - 1px) 100%, 0 100%)',
  right: 'polygon(calc(50% + 1px) 0, 100% 0, 100% 100%, calc(50% + 1px) 100%)',
}

const LINE = 'color-mix(in srgb, var(--text-dim) 45%, transparent)'
const EDGE = { boxShadow: `inset 0 0 0 1px ${LINE}` }

// A small window drawn in one theme's own colours: data-preview gives the theme's tokens to everything inside.
export function WireframePane({ theme, clip }: WireframePaneProps) {
  return (
    <span data-preview={theme} className="absolute inset-0" style={clip ? { clipPath: CLIPS[clip] } : undefined}>
      <span className="absolute inset-0 bg-app" />
      <span className="absolute inset-y-0 left-0 w-[22%] bg-surface" style={{ boxShadow: `inset -1px 0 0 ${LINE}` }} />
      <span className="absolute top-[8%] left-[3%] h-[8%] w-[16%] rounded-md bg-raised" style={EDGE} />
      <span className="absolute top-[22%] left-[3%] h-[7%] w-[16%] rounded-md bg-hover" />
      <span className="absolute top-[32%] left-[3%] h-[7%] w-[16%] rounded-md bg-hover opacity-70" />
      <span className="absolute top-[42%] left-[3%] h-[7%] w-[16%] rounded-md bg-hover opacity-50" />
      <span className="absolute top-[11%] right-[28%] h-[9%] w-[24%] rounded-lg bg-hover" />
      <span className="absolute top-[28%] left-[27%] h-[5%] w-[34%] rounded-sm" style={{ background: LINE }} />
      <span className="absolute top-[38%] left-[27%] h-[5%] w-[26%] rounded-sm" style={{ background: LINE }} />
      <span className="absolute right-[6%] bottom-[8%] left-[26%] flex h-[15%] items-center justify-between rounded-md bg-raised px-1" style={EDGE}>
        <span className="block h-[26%] w-[34%] rounded-full" style={{ background: LINE }} />
        <span className="block aspect-square h-[58%] rounded-full bg-accent" />
      </span>
      <span className="absolute top-[8%] right-[5%] h-[46%] w-[20%] rounded-lg bg-raised" style={EDGE}>
        {[0, 1, 2].map((row) => (
          <span key={row} className="absolute right-[11%] left-[11%] flex items-center gap-1" style={{ top: `${10 + row * 30}%`, height: '20%' }}>
            <span className={`block aspect-square h-[26%] rounded-full opacity-60 ${row === 1 ? 'bg-accent' : row === 0 ? 'bg-finished' : 'bg-idle'}`} />
            <span className="block h-[30%] w-[52%] rounded-sm" style={{ background: LINE }} />
          </span>
        ))}
      </span>
    </span>
  )
}
