import { GlobalOutlined } from '@ant-design/icons'
import { Select } from 'antd'
import { useI18n, type Locale } from '../i18n/I18nProvider'

export function LanguageSwitcher() {
  const { locale, setLocale } = useI18n()
  return (
    <Select
      size="small"
      value={locale}
      onChange={(value) => setLocale(value as Locale)}
      style={{ width: 120 }}
      aria-label="Language"
      options={[
        { value: 'en', label: 'English' },
        { value: 'zh', label: '中文' },
      ]}
      suffixIcon={<GlobalOutlined />}
    />
  )
}