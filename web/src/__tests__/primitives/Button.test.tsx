import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Button } from '../../components/primitives/Button'

describe('Button', () => {
  it('renders label and fires onClick', async () => {
    const onClick = vi.fn()
    render(<Button onClick={onClick}>Save</Button>)
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('respects disabled state', async () => {
    const onClick = vi.fn()
    render(<Button disabled onClick={onClick}>X</Button>)
    await userEvent.click(screen.getByRole('button'))
    expect(onClick).not.toHaveBeenCalled()
  })

  it.each(['default', 'primary', 'danger', 'ghost'] as const)(
    'renders %s variant',
    (variant) => {
      render(<Button variant={variant}>btn</Button>)
      expect(screen.getByRole('button', { name: 'btn' })).toBeInTheDocument()
    }
  )
})
