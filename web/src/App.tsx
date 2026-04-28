import { Bootstrap } from './Bootstrap'
import { Health } from './Health'

export function App(): JSX.Element {
  return (
    <>
      <Bootstrap />
      <h1 className="text-2xl font-semibold mb-4">mneme dashboard</h1>
      <p className="mb-4">Backend is up. Built UI is loaded.</p>
      <p>Daemon health: <Health /></p>
    </>
  )
}
