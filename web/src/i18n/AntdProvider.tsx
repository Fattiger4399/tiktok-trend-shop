import { App, ConfigProvider, theme } from 'antd'
import enUS from 'antd/locale/en_US'
import zhCN from 'antd/locale/zh_CN'
import { useI18n, type Locale } from './I18nProvider'
import type { ReactNode } from 'react'

export function AntdProvider({ children }: { children: ReactNode }) {
  const { locale } = useI18n()
  const antLocale = toAntdLocale(locale)
  return (
    <ConfigProvider
      locale={antLocale}
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          colorPrimary: '#2f6df6',
          borderRadius: 6,
          fontSize: 14,
        },
      }}
    >
      <App>{children}</App>
    </ConfigProvider>
  )
}

function toAntdLocale(locale: Locale) {
  return locale === 'zh' ? zhCN : enUS
}