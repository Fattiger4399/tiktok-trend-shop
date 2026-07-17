import type { ReactNode } from 'react'
import { I18nProvider } from '../i18n/I18nProvider'
import { AntdProvider } from '../i18n/AntdProvider'
import { MemoryRouter } from 'react-router-dom'

export function TestProviders({
  children,
  initialEntries,
}: {
  children: ReactNode
  initialEntries?: string[]
}) {
  return (
    <I18nProvider>
      <AntdProvider>
        <MemoryRouter initialEntries={initialEntries ?? ['/trends']}>{children}</MemoryRouter>
      </AntdProvider>
    </I18nProvider>
  )
}