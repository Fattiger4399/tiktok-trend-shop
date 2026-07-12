import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { useProduct, useProductMetrics, useCategories } from '../hooks/queries'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { assignProductCategory } from '../api/client'

export default function ProductDetailPage() {
  const { id } = useParams<{ id: string }>()
  const product = useProduct(id)
  const metrics = useProductMetrics(id, 'all')
  const categories = useCategories()
  const [selectedCategory, setSelectedCategory] = useState<string>('')
  const [feedback, setFeedback] = useState<string | null>(null)

  if (product.isLoading) return <LoadingState label="Loading product" rows={6} />
  if (product.error) return <ErrorState error={product.error as Error} />
  const detail = product.data
  if (!detail) return <EmptyState title="Product not found" />

  const handleAssign = async () => {
    if (!selectedCategory || !id) return
    try {
      await assignProductCategory(id, selectedCategory, 'workbench')
      setFeedback('Category assigned.')
      product.refetch()
    } catch (err) {
      setFeedback((err as Error).message)
    }
  }

  const history = (metrics.data?.items ?? []).map((item) => ({
    captured_at: new Date(item.captured_at).toLocaleDateString(),
    views: item.metrics.views ?? 0,
    sales: item.metrics.sales ?? 0,
    price: item.metrics.price ?? null,
    reviews: item.metrics.reviews ?? null,
  }))

  const priceSeries = history.filter((row) => row.price !== null)

  return (
    <section aria-labelledby="product-detail-title">
      <p>
        <Link to="/trends">← Back to trending</Link>
      </p>
      <h2 id="product-detail-title" className="card-title" style={{ margin: '12px 0' }}>
        {detail.product.title}
      </h2>
      <div className="muted" style={{ marginBottom: 16 }}>
        {detail.product.marketplace ? `${detail.product.marketplace} · ` : ''}
        {detail.product.asin ?? 'no ASIN'} · last update{' '}
        {detail.latest_snapshot?.captured_at
          ? new Date(detail.latest_snapshot.captured_at).toLocaleString()
          : 'unknown'}
        {detail.latest_snapshot?.metric_kind === 'estimated' ? (
          <span className="tag is-warning" style={{ marginLeft: 8 }}>Estimated metrics</span>
        ) : null}
      </div>

      <div className="layout-two-column">
        <div className="card">
          <div className="card-header">
            <h3 className="card-title">Product detail</h3>
            {detail.detail?.completeness === 'complete' ? (
              <span className="tag is-positive">Complete</span>
            ) : (
              <span className="tag is-warning">Incomplete</span>
            )}
          </div>
          {detail.detail ? (
            <dl>
              <dt>Price</dt>
              <dd>
                {detail.detail.price !== null && detail.detail.price !== undefined
                  ? `${detail.detail.currency ?? ''} ${detail.detail.price.toFixed(2)}`.trim()
                  : <span className="unavailable">unavailable</span>}
              </dd>
              <dt>Selling points</dt>
              <dd>
                {detail.detail.selling_points?.length
                  ? detail.detail.selling_points.join(', ')
                  : <span className="unavailable">unavailable</span>}
              </dd>
              <dt>Review summary</dt>
              <dd>
                {detail.detail.review_summary ?? <span className="unavailable">unavailable</span>}
              </dd>
              <dt>Shop</dt>
              <dd>
                {detail.detail.shop_name ?? <span className="unavailable">unavailable</span>}
                {detail.detail.brand ? ` · ${detail.detail.brand}` : ''}
              </dd>
              <dt>Missing fields</dt>
              <dd>
                {detail.detail.missing_fields?.length
                  ? detail.detail.missing_fields.join(', ')
                  : '—'}
              </dd>
            </dl>
          ) : (
            <EmptyState title="No detail snapshot recorded" />
          )}
        </div>

        <div className="card">
          <div className="card-header">
            <h3 className="card-title">Hotspot explanation</h3>
            {detail.score ? (
              <span className={`tag ${confidenceTag(detail.score.confidence)}`}>
                {detail.score.confidence} confidence
              </span>
            ) : null}
          </div>
          {detail.score ? (
            <>
              <p>
                Total score: <strong>{detail.score.total_score.toFixed(2)}</strong> · model {detail.score.model_version} · window {detail.score.time_window}
              </p>
              <ul>
                {Object.entries(detail.score.components).map(([key, value]) => (
                  <li key={key}>
                    <span>{key}</span>:{' '}
                    <strong>{(value as number).toFixed(2)}</strong>
                  </li>
                ))}
              </ul>
              {detail.score.missing_components?.length ? (
                <p className="muted">
                  Missing components: {detail.score.missing_components.join(', ')}
                </p>
              ) : null}
            </>
          ) : (
            <EmptyState title="No score snapshot available" description="Run scoring to compute one." />
          )}
          <hr style={{ margin: '12px 0' }} />
          <h4 className="card-title">Classification</h4>
          {detail.assignment ? (
            <p>
              <strong>{detail.assignment.canonical_category_id ?? 'unresolved'}</strong>{' '}
              <span className="muted">
                via {detail.assignment.method} ({Math.round(detail.assignment.confidence * 100)}%)
              </span>
            </p>
          ) : (
            <p className="muted">No classification recorded.</p>
          )}
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <select
              className="select"
              value={selectedCategory}
              onChange={(e) => setSelectedCategory(e.target.value)}
              aria-label="Assign canonical category"
            >
              <option value="">Choose canonical category…</option>
              {categories.data?.items?.map((cat) => (
                <option key={cat.id} value={cat.id}>{cat.name}</option>
              ))}
            </select>
            <button type="button" className="button" onClick={handleAssign} disabled={!selectedCategory}>
              Assign manually
            </button>
          </div>
          {feedback ? <p className="muted" role="status">{feedback}</p> : null}
        </div>
      </div>

      <div className="card" style={{ marginTop: 16 }}>
        <div className="card-header">
          <h3 className="card-title">Metric history</h3>
          {history.length < 2 ? (
            <span className="tag is-warning">Insufficient history</span>
          ) : null}
        </div>
        {history.length === 0 ? (
          <EmptyState title="No observations recorded yet" />
        ) : history.length < 2 ? (
          <p className="muted">
            At least two observations are required to draw a trend. Showing the latest known value.
          </p>
        ) : (
          <div style={{ display: 'grid', gap: 16 }}>
            <ResponsiveContainer width="100%" height={200}>
              <LineChart data={history}>
                <CartesianGrid stroke="#e5e7eb" strokeDasharray="3 3" />
                <XAxis dataKey="captured_at" />
                <YAxis />
                <Tooltip />
                <Line type="monotone" dataKey="views" stroke="#2f6df6" />
                <Line type="monotone" dataKey="sales" stroke="#16a34a" />
              </LineChart>
            </ResponsiveContainer>
            {priceSeries.length >= 2 ? (
              <ResponsiveContainer width="100%" height={160}>
                <LineChart data={priceSeries}>
                  <CartesianGrid stroke="#e5e7eb" strokeDasharray="3 3" />
                  <XAxis dataKey="captured_at" />
                  <YAxis />
                  <Tooltip />
                  <Line type="monotone" dataKey="price" stroke="#d97706" />
                </LineChart>
              </ResponsiveContainer>
            ) : null}
          </div>
        )}
      </div>
    </section>
  )
}

function confidenceTag(confidence: string): string {
  if (confidence === 'high') return 'is-positive'
  if (confidence === 'medium') return 'is-warning'
  return 'is-negative'
}