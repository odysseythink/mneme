import { forwardRef, type InputHTMLAttributes } from 'react'

type Props = InputHTMLAttributes<HTMLInputElement> & { mono?: boolean }

export const Input = forwardRef<HTMLInputElement, Props>(function Input(
  { mono, style, ...rest },
  ref,
) {
  return (
    <input
      ref={ref}
      {...rest}
      style={{
        padding: '5px 10px',
        borderRadius: 'var(--radius-2)',
        border: '1px solid var(--border-default)',
        background: 'var(--bg-base)',
        color: 'var(--text-body)',
        fontSize: 12,
        fontFamily: mono ? 'var(--font-mono)' : 'var(--font-sans)',
        outline: 'none',
        ...style,
      }}
      onFocus={(e) => {
        e.currentTarget.style.borderColor = 'var(--accent)'
        e.currentTarget.style.boxShadow = '0 0 0 2px color-mix(in srgb, var(--accent) 15%, transparent)'
        rest.onFocus?.(e)
      }}
      onBlur={(e) => {
        e.currentTarget.style.borderColor = 'var(--border-default)'
        e.currentTarget.style.boxShadow = 'none'
        rest.onBlur?.(e)
      }}
    />
  )
})
