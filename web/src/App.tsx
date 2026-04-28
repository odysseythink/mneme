import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { Overview } from './panels/Overview'
import { Activity } from './panels/Activity'
import { Cron } from './panels/Cron'
import { Bootstrap } from './Bootstrap'
import { SSEProvider } from './hooks/useSSE'

const router = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <Overview /> },
      { path: 'activity', element: <Activity /> },
      { path: 'cron', element: <Cron /> },
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
