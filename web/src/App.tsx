import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { Overview } from './panels/Overview'
import { Activity } from './panels/Activity'
import { Cron } from './panels/Cron'
import { Cerebrum } from './panels/Cerebrum'
import { Memory } from './panels/Memory'
import { Anatomy } from './panels/Anatomy'
import { BugLog } from './panels/BugLog'
import { Suggestions } from './panels/Suggestions'
import { Token } from './panels/Token'
import { DesignQC } from './panels/DesignQC'
import { Bootstrap } from './Bootstrap'
import { SSEProvider } from './hooks/useSSE'
import { ActiveProjectProvider } from './hooks/useActiveProject'

const router = createBrowserRouter([
  {
    path: '/',
    element: <ActiveProjectProvider><AppShell /></ActiveProjectProvider>,
    children: [
      { index: true, element: <Overview /> },
      { path: 'activity', element: <Activity /> },
      { path: 'cron', element: <Cron /> },
      { path: 'cerebrum', element: <Cerebrum /> },
      { path: 'memory', element: <Memory /> },
      { path: 'anatomy', element: <Anatomy /> },
      { path: 'buglog', element: <BugLog /> },
      { path: 'suggestions', element: <Suggestions /> },
      { path: 'token', element: <Token /> },
      { path: 'designqc', element: <DesignQC /> },
    ],
  },
])

export function App(): JSX.Element {
  return (
    <>
      <Bootstrap />
      <SSEProvider>
        <RouterProvider router={router} />
      </SSEProvider>
    </>
  )
}
