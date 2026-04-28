import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Pill } from '../../components/primitives/Pill'

describe('Pill', () => {
  it('renders children', () => {
    render(<Pill variant="write">WRITE</Pill>)
    expect(screen.getByText('WRITE')).toBeInTheDocument()
  })

  it.each(['write', 'read', 'warn', 'err', 'info', 'neutral'] as const)(
    'applies %s variant color',
    (variant) => {
      render(<Pill variant={variant}>{variant}</Pill>)
      const el = screen.getByText(variant)
      expect(el.style.color).toMatch(/var\(--/)
    }
  )

  it('renders solid variant with accent background', () => {
    render(<Pill variant="solid">LIVE</Pill>)
    expect(screen.getByText('LIVE').style.background).toContain('var(--accent)')
  })
})
