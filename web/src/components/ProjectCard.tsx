import type { ProjectSummary } from '../api/types'

export function ProjectCard({ p }: { p: ProjectSummary }): JSX.Element {
  return (
    <div className="rounded border p-3">
      <div className="font-medium">{p.origin || p.id}</div>
      <div className="text-sm text-gray-600 mt-1">
        {p.anatomy_files} files · {p.cerebrum_pending} cerebrum pending · {Math.round(p.memory_bytes / 1024)} KB memory
      </div>
    </div>
  )
}
