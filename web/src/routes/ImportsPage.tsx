import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useImports } from '../hooks/queries'
import { createImport } from '../api/client'
import { EmptyState, ErrorState, LoadingState } from '../components/States'

const MAX_BYTES = 10 * 1024 * 1024

export default function ImportsPage() {
  const imports = useImports(50)
  const [file, setFile] = useState<File | null>(null)
  const [region, setRegion] = useState<string>('CN')
  const [submitting, setSubmitting] = useState(false)
  const [feedback, setFeedback] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!file) {
      setError('Please select a CSV file before submitting.')
      return
    }
    if (file.size > MAX_BYTES) {
      setError(`File exceeds the ${(MAX_BYTES / 1024 / 1024).toFixed(0)} MB limit.`)
      return
    }
    setError(null)
    setSubmitting(true)
    try {
      const result = await createImport({ file, region })
      setFeedback(`Import job ${result.job_id} accepted (${result.imported_rows} imported).`)
      setFile(null)
      imports.refetch()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section>
      <h2 className="card-title" style={{ marginBottom: 16 }}>CSV Imports</h2>
      <form className="card" onSubmit={submit} aria-label="Upload CSV">
        <div className="card-header">
          <h3 className="card-title">Upload a new file</h3>
        </div>
        <div style={{ display: 'grid', gap: 12, gridTemplateColumns: '1fr 160px auto' }}>
          <input
            type="file"
            accept=".csv,text/csv"
            className="input"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            aria-label="CSV file"
          />
          <select className="select" value={region} onChange={(e) => setRegion(e.target.value)} aria-label="Default region">
            <option value="CN">CN</option>
            <option value="US">US</option>
            <option value="UK">UK</option>
            <option value="JP">JP</option>
            <option value="DE">DE</option>
          </select>
          <button className="button" type="submit" disabled={submitting}>
            {submitting ? 'Uploading…' : 'Upload'}
          </button>
        </div>
        {error ? <p className="tag is-negative" role="alert">{error}</p> : null}
        {feedback ? <p className="tag is-positive" role="status">{feedback}</p> : null}
        <p className="muted" style={{ marginTop: 8 }}>
          Files must include a <code>title</code> column. Optional columns: <code>marketplace</code>,{' '}
          <code>asin</code>, <code>category</code>, <code>price</code>, <code>selling_points</code>, <code>specs</code>,{' '}
          <code>review_summary</code>, <code>views</code>, <code>sales</code>, etc.
        </p>
      </form>

      <div className="card" style={{ marginTop: 16 }}>
        <div className="card-header">
          <h3 className="card-title">Recent imports</h3>
        </div>
        {imports.isLoading ? (
          <LoadingState label="Loading import history" rows={3} />
        ) : imports.error ? (
          <ErrorState error={imports.error as Error} />
        ) : (imports.data?.items ?? []).length === 0 ? (
          <EmptyState title="No imports yet" description="Upload a CSV to create your first import job." />
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>File</th>
                <th>Status</th>
                <th>Imported</th>
                <th>Updated</th>
                <th>Rejected</th>
                <th>Started</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {(imports.data?.items ?? []).map((job) => (
                <tr key={job.id}>
                  <td>{job.filename ?? <span className="unavailable">unnamed</span>}</td>
                  <td>
                    <span className={`tag ${job.status === 'completed' ? 'is-positive' : job.status === 'failed' ? 'is-negative' : 'is-warning'}`}>
                      {job.status}
                    </span>
                  </td>
                  <td>{job.imported_rows}</td>
                  <td>{job.updated_rows}</td>
                  <td>{job.rejected_rows}</td>
                  <td>{new Date(job.started_at).toLocaleString()}</td>
                  <td><Link to={`/imports/${job.id}`}>View</Link></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  )
}