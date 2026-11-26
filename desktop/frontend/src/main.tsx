import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { OrchestratorProvider } from './OrchestratorContext'
import { LayoutProvider } from './LayoutContext'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <OrchestratorProvider>
      <LayoutProvider>
        <App />
      </LayoutProvider>
    </OrchestratorProvider>
  </StrictMode>,
)
