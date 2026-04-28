import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { PageHead } from '../components/PageHead'

describe('PageHead', () => {
  it('renders title only', () => {
    render(<PageHead title="Overview" />)
    expect(screen.getByText('Overview')).toBeInTheDocument()
  })

  it('renders title + meta', () => {
    render(<PageHead title="Activity" meta="window=24h" />)
    expect(screen.getByText('window=24h')).toBeInTheDocument()
  })

  it('renders title + actions', () => {
    render(<PageHead title="X" actions={<button>Scan</button>} />)
    expect(screen.getByRole('button', { name: 'Scan' })).toBeInTheDocument()
  })
})
