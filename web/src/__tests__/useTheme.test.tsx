import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useTheme } from '../hooks/useTheme'

describe('useTheme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
  })
  afterEach(() => { vi.restoreAllMocks() })

  function mockMatchMedia(prefersDark: boolean) {
    const listeners: ((e: MediaQueryListEvent) => void)[] = []
    window.matchMedia = vi.fn().mockReturnValue({
      matches: prefersDark,
      addEventListener: (_: string, fn: (e: MediaQueryListEvent) => void) => listeners.push(fn),
      removeEventListener: vi.fn(),
    })
    return { fireChange: (next: boolean) => listeners.forEach(fn => fn({ matches: next } as MediaQueryListEvent)) }
  }

  it('defaults to "system" when nothing in storage', () => {
    mockMatchMedia(true)
    const { result } = renderHook(() => useTheme())
    expect(result.current.preference).toBe('system')
    expect(result.current.resolved).toBe('dark')
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
  })

  it('reads stored "light" preference', () => {
    localStorage.setItem('mneme.theme', 'light')
    mockMatchMedia(true)
    const { result } = renderHook(() => useTheme())
    expect(result.current.preference).toBe('light')
    expect(result.current.resolved).toBe('light')
  })

  it('cycles dark → light → system on setNext', () => {
    localStorage.setItem('mneme.theme', 'dark')
    mockMatchMedia(false)
    const { result } = renderHook(() => useTheme())
    expect(result.current.preference).toBe('dark')
    act(() => result.current.setNext())
    expect(result.current.preference).toBe('light')
    act(() => result.current.setNext())
    expect(result.current.preference).toBe('system')
    act(() => result.current.setNext())
    expect(result.current.preference).toBe('dark')
  })

  it('persists preference to localStorage', () => {
    mockMatchMedia(true)
    const { result } = renderHook(() => useTheme())
    act(() => result.current.setPreference('light'))
    expect(localStorage.getItem('mneme.theme')).toBe('light')
  })

  it('reacts to OS theme change in system mode', () => {
    const mq = mockMatchMedia(false)
    const { result } = renderHook(() => useTheme())
    expect(result.current.resolved).toBe('light')
    act(() => mq.fireChange(true))
    expect(result.current.resolved).toBe('dark')
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
  })
})
