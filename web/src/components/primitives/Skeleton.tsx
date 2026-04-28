export function Skeleton({ rows = 3 }: { rows?: number }): JSX.Element {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      {Array.from({ length: rows }, (_, i) => (
        <div
          key={i}
          style={{
            height: 28,
            borderRadius: 'var(--radius-2)',
            background:
              'linear-gradient(90deg, var(--bg-raised) 0%, var(--bg-surface) 50%, var(--bg-raised) 100%)',
            backgroundSize: '200% 100%',
            animation: 'shimmer 1.5s linear infinite',
          }}
        />
      ))}
    </div>
  )
}
