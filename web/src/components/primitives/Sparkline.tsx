type Props = {
  data: number[]
  height?: number
  stroke?: string  /* override; otherwise auto by trend */
}

export function Sparkline({ data, height = 18, stroke }: Props): JSX.Element | null {
  if (data.length === 0) return null
  const min = Math.min(...data)
  const max = Math.max(...data)
  const range = max - min || 1
  const stepX = data.length === 1 ? 0 : 100 / (data.length - 1)
  const points = data
    .map((v, i) => `${(i * stepX).toFixed(2)},${(height - ((v - min) / range) * height).toFixed(2)}`)
    .join(' ')
  const trend = data[data.length - 1] - data[0]
  const auto = trend >= 0 ? 'var(--ok)' : 'var(--err)'
  return (
    <svg
      viewBox={`0 0 100 ${height}`}
      preserveAspectRatio="none"
      style={{ width: '100%', height }}
    >
      <polyline fill="none" stroke={stroke ?? auto} strokeWidth="1" points={points} />
    </svg>
  )
}
