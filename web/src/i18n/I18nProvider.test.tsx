import { describe, expect, it } from 'vitest'
import { renderHook } from '@testing-library/react'
import { I18nProvider, useI18n } from '../i18n/I18nProvider'

describe('i18n', () => {
  it('translates English by default', () => {
    const { result } = renderHook(() => useI18n(), { wrapper: I18nProvider })
    expect(result.current.t('app.brand')).toBe('Trend Workbench')
    expect(result.current.t('trends.title')).toBe('Trending Products')
  })

  it('switches to Chinese and back', () => {
    const { result } = renderHook(() => useI18n(), { wrapper: I18nProvider })
    actSetLocale(result.current.setLocale, 'zh')
    expect(result.current.locale).toBe('zh')
    expect(result.current.t('app.brand')).toBe('趋势工作台')
    expect(result.current.t('trends.title')).toBe('热门商品')
    actSetLocale(result.current.setLocale, 'en')
    expect(result.current.t('app.brand')).toBe('Trend Workbench')
  })

  it('interpolates parameters', () => {
    const { result } = renderHook(() => useI18n(), { wrapper: I18nProvider })
    expect(result.current.t('imports.success', { id: 'abc', count: 3 })).toContain('abc')
  })
})

import { act } from '@testing-library/react'

function actSetLocale(set: (l: 'en' | 'zh') => void, locale: 'en' | 'zh') {
  act(() => set(locale))
}