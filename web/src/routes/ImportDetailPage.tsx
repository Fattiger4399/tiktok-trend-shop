import { Link, useParams } from 'react-router-dom'
import { useImportDetail } from '../hooks/queries'
import { EmptyState, ErrorState, LoadingState } from '../components/States'

export default function ImportDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { data, isLoading, error } = useImportDetail(id)
  if (isLoading) return <LoadingState label="Loading import" rows={3} />
  if (error) return <ErrorState error={error as Error} />
  if (!data) return <EmptyState title="Import not found" />
  const { job, results } = data
  return (
    <section>
      <p><Link to="/imports">← Back to imports</Link></p>
      <h2 className="card-title" style={{ margin: '12px 0' }}>
        Import {job.id}
      </h2>
      <p className="muted">
        {job.filename ?? 'unnamed file'} · status <strong>{job.status}</strong>
      </p>
      <div className="layout-two-column">
        <div className="card">
          <h3 className="card-title">Summary</h3>
          <dl>
            <dt>Imported rows</dt><dd>{job.imported_rows}</dd>
            <dt>Updated rows</dt><dd>{job.updated_rows}</dd>
            <dt>Duplicate rows</dt><dd>{job.duplicate_rows}</dd>
            <dt>Rejected rows</dt><dd>{job.rejected_rows}</dd>
            <dt>Started at</dt><dd>{new Date(job.started_at).toLocaleString()}</dd>
            <dt>Completed at</dt><dd>{job.completed_at ? new Date(job.completed_at).toLocaleString() : '—'}</dd>
          </dl>
        </div>
        <div className="card">
          <h3 className="card-title">Row outcomes</h3>
          {results.length === 0 ? (
            <p className="muted">No row outcomes recorded.</p>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Row</th>
                  <th>Status</th>
                  <th>Reason</th>
                </tr>
              </thead>
              <tbody>
                {results.map((row) => (
                  <tr key={`${row.row_number}-${row.status}`}>
                    <td>{row.row_number}</td>
                    <td>{row.status}</td>
                    <td>{row.reason ?? '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </section>
  )
}