import { Alert, Button, Empty, Skeleton, Space } from 'antd'
import type { ReactNode } from 'react'

export interface LoadingStateProps {
  label?: string
  rows?: number
}

export function LoadingState({ label = 'Loading', rows = 4 }: LoadingStateProps) {
  return (
    <div role="status" aria-live="polite" style={{ padding: '16px 0' }}>
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <span style={{ color: '#6b7280' }}>{label}</span>
        <Skeleton active paragraph={{ rows }} />
      </Space>
    </div>
  )
}

export interface EmptyStateProps {
  title: string
  description?: string
  action?: ReactNode
}

export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <Empty
      description={
        <Space direction="vertical" size={4}>
          <span style={{ fontWeight: 600 }}>{title}</span>
          {description ? <span style={{ color: '#6b7280' }}>{description}</span> : null}
        </Space>
      }
    >
      {action}
    </Empty>
  )
}

export interface ErrorStateProps {
  error: Error
  onRetry?: () => void
}

export function ErrorState({ error, onRetry }: ErrorStateProps) {
  return (
    <Alert
      type="error"
      showIcon
      message="Something went wrong."
      description={error.message}
      action={
        onRetry ? (
          <Button size="small" onClick={onRetry}>
            Retry
          </Button>
        ) : null
      }
    />
  )
}