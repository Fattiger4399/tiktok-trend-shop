import '../styles/global.css'

export interface LoadingStateProps {
  label?: string
  rows?: number
}

export function LoadingState({ label = 'Loading', rows = 4 }: LoadingStateProps) {
  return (
    <div className="state-loading" role="status" aria-live="polite">
      <span className="state-loading-label">{label}</span>
      <div className="state-loading-skeletons">
        {Array.from({ length: rows }).map((_, idx) => (
          <div key={idx} className="state-loading-skeleton" />
        ))}
      </div>
    </div>
  )
}

export interface EmptyStateProps {
  title: string
  description?: string
  action?: React.ReactNode
}

export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <div className="state-empty" role="status">
      <p className="state-empty-title">{title}</p>
      {description ? <p className="state-empty-description">{description}</p> : null}
      {action ? <div className="state-empty-action">{action}</div> : null}
    </div>
  )
}

export interface ErrorStateProps {
  error: Error
  onRetry?: () => void
}

export function ErrorState({ error, onRetry }: ErrorStateProps) {
  return (
    <div className="state-error" role="alert">
      <p className="state-error-title">Something went wrong.</p>
      <p className="state-error-message">{error.message}</p>
      {onRetry ? (
        <button type="button" className="state-error-retry" onClick={onRetry}>
          Retry
        </button>
      ) : null}
    </div>
  )
}