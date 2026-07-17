import { describe, expect, it } from 'vitest'
import { useTrendsURL } from '../hooks/useTrendsURL'
import { renderHook, act } from '@testing-library/react'
import { TestProviders } from '../test/TestProviders'

describe('useTrendsURL', () => {
  it('defaults sort to score and direction to desc', () => {
    const { result } = renderHook(() => useTrendsURL(), {
      wrapper: ({ children }) => <TestProviders>{children}</TestProviders>,
    })
    expect(result.current.params.sort).toBe('score')
    expect(result.current.params.direction).toBe('desc')
    expect(result.current.params.window).toBe('30d')
  })

  it('updates parameters without losing state', () => {
    const { result } = renderHook(() => useTrendsURL(), {
      wrapper: ({ children }) => <TestProviders>{children}</TestProviders>,
    })
    act(() => result.current.setParam('q', 'lamp'))
    expect(result.current.params.q).toBe('lamp')
  })

  it('clears all parameters', () => {
    const { result } = renderHook(() => useTrendsURL(), {
      wrapper: ({ children }) => <TestProviders>{children}</TestProviders>,
    })
    act(() => result.current.setParam('q', 'lamp'))
    act(() => result.current.clear())
    expect(result.current.params.q).toBe('')
  })
})