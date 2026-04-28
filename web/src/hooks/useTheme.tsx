import { useCallback, useEffect, useState } from 'react'

export type ThemePreference = 'dark' | 'light' | 'system'
export type ThemeResolved = 'dark' | 'light'

const STORAGE_KEY = 'mneme.theme'
const CYCLE: ThemePreference[] = ['dark', 'light', 'system']

function readPref(): ThemePreference {
  const raw = localStorage.getItem(STORAGE_KEY)
  return raw === 'dark' || raw === 'light' || raw === 'system' ? raw : 'system'
}

function resolve(pref: ThemePreference): ThemeResolved {
  if (pref === 'dark' || pref === 'light') return pref
  return matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function useTheme() {
  const [preference, setPreferenceState] = useState<ThemePreference>(readPref)
  const [resolved, setResolved] = useState<ThemeResolved>(() => resolve(readPref()))

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', resolved)
  }, [resolved])

  useEffect(() => {
    if (preference !== 'system') {
      setResolved(preference)
      return
    }
    const mq = matchMedia('(prefers-color-scheme: dark)')
    const update = (e: MediaQueryListEvent) => setResolved(e.matches ? 'dark' : 'light')
    setResolved(mq.matches ? 'dark' : 'light')
    mq.addEventListener('change', update)
    return () => mq.removeEventListener('change', update)
  }, [preference])

  const setPreference = useCallback((p: ThemePreference) => {
    localStorage.setItem(STORAGE_KEY, p)
    setPreferenceState(p)
  }, [])

  const setNext = useCallback(() => {
    setPreferenceState(prev => {
      const next = CYCLE[(CYCLE.indexOf(prev) + 1) % CYCLE.length]
      localStorage.setItem(STORAGE_KEY, next)
      return next
    })
  }, [])

  return { preference, resolved, setPreference, setNext }
}
