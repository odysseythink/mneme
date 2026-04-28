type SparklineProps = {
  data: number[]
  width?: number
  height?: number
  color?: string
}

export function Sparkline({ data, width = 120, height = 30, color = '#2563eb' }: SparklineProps): JSX.Element {
  if (data.length < 2) return <span style={{ display: 'inline-block', width, height }} />
  const max = Math.max(...data)
  const min = Math.min(...data)
  const range = max - min || 1
  const stepX = width / (data.length - 1)
  const points = data
    .map((v, i) => `${i * stepX},${height - ((v - min) / range) * height}`)
    .join(' ')
  return (
    <svg width={width} height={height} className="overflow-visible">
      <polyline points={points} fill="none" stroke={color} strokeWidth={1.5} />
    </svg>
  )
}
