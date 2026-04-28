import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Stat } from '../../components/primitives/Stat'

describe('Stat', () => {
  it('renders label and value', () => {
    render(<Stat label="Turns 24h" value="1,284" />)
    expect(screen.getByText('Turns 24h')).toBeInTheDocument()
    expect(screen.getByText('1,284')).toBeInTheDocument()
  })

  it('renders delta with up arrow when positive', () => {
    render(<Stat label="Turns" value="1,284" delta={{ value: '12%', direction: 'up' }} />)
    const delta = screen.getByText(/12%/)
    expect(delta.textContent).toContain('↑')
  })

  it('renders delta with down arrow when negative', () => {
    render(<Stat label="Tokens" value="847k" delta={{ value: '3%', direction: 'down' }} />)
    expect(screen.getByText(/3%/).textContent).toContain('↓')
  })

  it('renders breakdown text when provided', () => {
    render(<Stat label="Bugs" value="3" breakdown="2 medium · 1 high" />)
    expect(screen.getByText('2 medium · 1 high')).toBeInTheDocument()
  })
})
