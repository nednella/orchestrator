import { Component } from 'react'
import type { ReactNode } from 'react'
import { Notice } from './Notice'

type CentreBoundaryProps = { children: ReactNode }
type CentreBoundaryState = { failed: boolean }

// CentreBoundary is the one class in the app: React has no hook that catches a render error. A view that throws shows
// a Notice with Reload in its place, so the rest of the window stays up.
export class CentreBoundary extends Component<CentreBoundaryProps, CentreBoundaryState> {
  state: CentreBoundaryState = { failed: false }

  static getDerivedStateFromError(): CentreBoundaryState {
    return { failed: true }
  }

  componentDidCatch(error: unknown) {
    console.error(error)
  }

  render() {
    if (!this.state.failed) return this.props.children
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <Notice title="This view stopped working" hint="Reload to start it again." />
        <button className="btn" onClick={() => window.location.reload()}>
          Reload
        </button>
      </div>
    )
  }
}
