import { Tag } from 'antd'
import { useI18n } from '../i18n/I18nProvider'
import type { MaterialRequestStatus } from '../api/types'

const STATUS_COLORS: Record<MaterialRequestStatus, string> = {
  submitted: 'blue',
  generating: 'gold',
  generated: 'cyan',
  approved: 'green',
  rejected: 'red',
  delivered: 'purple',
}

export function requestStatusColor(status: string): string {
  return (STATUS_COLORS as Record<string, string>)[status] ?? 'default'
}

export function RequestStatusTag({ status }: { status: string }) {
  const { t } = useI18n()
  return (
    <Tag color={requestStatusColor(status)}>
      {t(`requests.status.${status}` as 'requests.status.submitted')}
    </Tag>
  )
}
