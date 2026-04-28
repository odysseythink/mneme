import type { ButtonHTMLAttributes, ReactNode } from 'react'

export type ButtonVariant = 'default' | 'primary' | 'danger' | 'ghost'
export type ButtonSize = 'sm' | 'md' | 'icon'

type Props = Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> & {
  variant?: ButtonVariant
  size?: ButtonSize
  children: ReactNode
}

const VARIANT_STYLE: Record<ButtonVariant, React.CSSProperties> = {
  default: {
    background: 'var(--bg-base)',
    color: 'var(--text-body)',
    border: '1px solid var(--border-default)',
  },
  primary: {
    background: 'var(--accent)',
    color: 'var(--bg-base)',
    border: '1px solid var(--accent)',
    fontWeight: 500,
  },
  danger: {
    background: 'transparent',
    color: 'var(--err)',
    border: '1px solid color-mix(in srgb, var(--err) 30%, transparent)',
  },
  ghost: {
    background: 'transparent',
    color: 'var(--text-muted)',
    border: '1px solid transparent',
  },
}

const SIZE_STYLE: Record<ButtonSize, React.CSSProperties> = {
  md: { padding: '5px 12px', fontSize: 12 },
  sm: { padding: '2px 8px', fontSize: 11 },
  icon: { width: 26, height: 26, padding: 0, fontSize: 14 },
}

export function Button({ variant = 'default', size = 'md', style, ...rest }: Props): JSX.Element {
  return (
    <button
      {...rest}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 6,
        borderRadius: 'var(--radius-2)',
        fontFamily: 'var(--font-sans)',
        cursor: rest.disabled ? 'not-allowed' : 'pointer',
        opacity: rest.disabled ? 0.5 : 1,
        ...VARIANT_STYLE[variant],
        ...SIZE_STYLE[size],
        ...style,
      }}
    />
  )
}
