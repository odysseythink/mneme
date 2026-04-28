import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Modal } from '../../components/primitives/Modal'

describe('Modal', () => {
  it('renders title, body, and actions when open', () => {
    render(
      <Modal open onClose={() => {}} title="Confirm">
        <div>Are you sure?</div>
      </Modal>,
    )
    expect(screen.getByText('Confirm')).toBeInTheDocument()
    expect(screen.getByText('Are you sure?')).toBeInTheDocument()
  })

  it('renders nothing when closed', () => {
    render(
      <Modal open={false} onClose={() => {}} title="Hidden">
        <div>body</div>
      </Modal>,
    )
    expect(screen.queryByText('Hidden')).not.toBeInTheDocument()
  })

  it('calls onClose when backdrop clicked', async () => {
    const onClose = vi.fn()
    render(
      <Modal open onClose={onClose} title="x">
        <div>body</div>
      </Modal>,
    )
    await userEvent.click(screen.getByTestId('modal-backdrop'))
    expect(onClose).toHaveBeenCalled()
  })

  it('does NOT call onClose when modal body clicked', async () => {
    const onClose = vi.fn()
    render(
      <Modal open onClose={onClose} title="x">
        <div>body</div>
      </Modal>,
    )
    await userEvent.click(screen.getByText('body'))
    expect(onClose).not.toHaveBeenCalled()
  })
})
