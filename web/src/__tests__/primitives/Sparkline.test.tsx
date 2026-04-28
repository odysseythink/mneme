import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Sparkline } from '../../components/primitives/Sparkline'

describe('Sparkline', () => {
  it('renders a polyline with one point per data value', () => {
    const { container } = render(<Sparkline data={[1, 2, 3, 4, 5]} />)
    const polyline = container.querySelector('polyline')
    expect(polyline).not.toBeNull()
    const points = polyline!.getAttribute('points')!.split(' ')
    expect(points).toHaveLength(5)
  })

  it('renders nothing when data is empty', () => {
    const { container } = render(<Sparkline data={[]} />)
    expect(container.querySelector('polyline')).toBeNull()
  })

  it('uses up color when last value > first value', () => {
    const { container } = render(<Sparkline data={[1, 5]} />)
    expect(container.querySelector('polyline')!.getAttribute('stroke')).toBe('var(--ok)')
  })

  it('uses down color when last value < first value', () => {
    const { container } = render(<Sparkline data={[5, 1]} />)
    expect(container.querySelector('polyline')!.getAttribute('stroke')).toBe('var(--err)')
  })
})
