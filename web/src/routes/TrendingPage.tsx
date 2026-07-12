import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useTrends } from '../hooks/queries'
import { useTrendsURL } from '../hooks/useTrendsURL'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { ProductRow } from '../components/ProductRow'

const SORT_OPTIONS: Array<{ value: string; label: string }> = [
  { value: 'score', label: 'Hotspot score' },
  { value: 'updated', label: 'Last update' },
  { value: 'price', label: 'Price' },
  { value: 'created', label: 'Discovered' },
  { value: 'asin', label: 'ASIN' },
]

const WINDOWS: Array<{ value: string; label: string }> = [
  { value: '24h', label: 'Last 24h' },
  { value: '7d', label: 'Last 7 days' },
  { value: '30d', label: 'Last 30 days' },
  { value: '90d', label: 'Last 90 days' },
  { value: 'all', label: 'All time' },
]

export default function TrendingPage() {
  const { params, setParam, clear } = useTrendsURL()
  const { data, error, isLoading, isFetching, refetch } = useTrends(params)
  const [searchInput, setSearchInput] = useState(params.q ?? '')

  const products = data?.items ?? []
  const effective = (data?.effective ?? {}) as Record<string, unknown>

  const sortLabel = useMemo(() => {
    const value = (effective.sort as string) ?? params.sort ?? 'score'
    return SORT_OPTIONS.find((opt) => opt.value === value)?.label ?? value
  }, [effective.sort, params.sort])

  return (
    <section aria-labelledby="trends-title">
      <h2 id="trends-title" className="card-title" style={{ marginBottom: 16 }}>
        Trending Products
      </h2>
      <div className="toolbar" role="toolbar" aria-label="Filters">
        <select
          className="select"
          value={params.marketplace ?? ''}
          onChange={(e) => setParam('marketplace', e.target.value)}
          aria-label="Marketplace"
        >
          <option value="">All marketplaces</option>
          <option value="US">US</option>
          <option value="UK">UK</option>
          <option value="DE">DE</option>
          <option value="JP">JP</option>
        </select>
        <select
          className="select"
          value={params.category ?? ''}
          onChange={(e) => setParam('category', e.target.value)}
          aria-label="Canonical category"
        >
          <option value="">All categories</option>
          <option value="cat-beauty">Beauty</option>
          <option value="cat-home">Home & Kitchen</option>
          <option value="cat-electronics">Electronics</option>
          <option value="cat-fashion">Fashion</option>
          <option value="cat-toys">Toys & Games</option>
          <option value="cat-sports">Sports</option>
        </select>
        <select
          className="select"
          value={params.window ?? '30d'}
          onChange={(e) => setParam('window', e.target.value as typeof params.window)}
          aria-label="Time window"
        >
          {WINDOWS.map((opt) => (
            <option key={opt.value} value={opt.value}>{opt.label}</option>
          ))}
        </select>
        <select
          className="select"
          value={params.sort ?? 'score'}
          onChange={(e) => setParam('sort', e.target.value as typeof params.sort)}
          aria-label="Sort field"
        >
          {SORT_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>{opt.label}</option>
          ))}
        </select>
        <select
          className="select"
          value={params.direction ?? 'desc'}
          onChange={(e) => setParam('direction', e.target.value as 'asc' | 'desc')}
          aria-label="Sort direction"
        >
          <option value="desc">Descending</option>
          <option value="asc">Ascending</option>
        </select>
        <form
          className="toolbar-grow"
          onSubmit={(e) => {
            e.preventDefault()
            setParam('q', searchInput)
          }}
          role="search"
        >
          <input
            className="input"
            type="search"
            placeholder="Search title or ASIN"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            aria-label="Search products"
          />
        </form>
        <button type="button" className="button is-secondary" onClick={clear}>
          Clear filters
        </button>
      </div>

      {isLoading ? (
        <LoadingState label="Loading trending products" rows={6} />
      ) : error ? (
        <ErrorState error={error as Error} onRetry={() => refetch()} />
      ) : products.length === 0 ? (
        <EmptyState
          title="No products match these filters"
          description="Try widening the time window or clearing filters."
        />
      ) : (
        <>
          <p className="muted" style={{ marginBottom: 8 }} aria-live="polite">
            Showing {products.length} products sorted by {sortLabel.toLowerCase()}
            {isFetching ? ' · refreshing…' : ''}
          </p>
          <table className="table" role="table" aria-label="Trending products">
            <thead>
              <tr>
                <th scope="col">Product</th>
                <th scope="col">Price</th>
                <th scope="col">Score</th>
                <th scope="col">Source</th>
                <th scope="col">Last update</th>
              </tr>
            </thead>
            <tbody>
              {products.map((product) => (
                <tr key={product.id}>
                  <td>
                    <Link to={`/products/${product.id}`}>{product.title}</Link>
                    <div className="muted">
                      {product.asin ?? 'no ASIN'}
                      {product.marketplace ? ` · ${product.marketplace}` : ''}
                    </div>
                  </td>
                  <td>
                    {product.price !== undefined && product.price !== null
                      ? `${product.currency ?? ''} ${product.price.toFixed(2)}`.trim()
                      : <span className="unavailable">unavailable</span>}
                  </td>
                  <td>
                    {product.score
                      ? `${product.score.total_score.toFixed(2)} (${product.score.confidence})`
                      : <span className="unavailable">unavailable</span>}
                  </td>
                  <td>
                    {product.provenance?.provider ?? <span className="unavailable">unknown</span>}
                  </td>
                  <td>
                    {product.freshness?.last_captured_at
                      ? new Date(product.freshness.last_captured_at).toLocaleString()
                      : <span className="unavailable">unknown</span>}
                    {product.freshness?.stale ? (
                      <div><span className="tag is-warning">Stale</span></div>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          <div className="mobile-only" style={{ marginTop: 16 }}>
            {products.map((product) => (
              <div key={`mobile-${product.id}`} style={{ marginBottom: 12 }}>
                <ProductRow product={product} />
              </div>
            ))}
          </div>

          {data && data.pagination.total_pages > 1 ? (
            <div style={{ display: 'flex', gap: 8, marginTop: 16 }} aria-label="Pagination">
              <button
                type="button"
                className="button is-secondary"
                disabled={!data.pagination.has_previous}
                onClick={() => setParam('page', Math.max(1, (params.page ?? 1) - 1))}
              >
                Previous
              </button>
              <span className="muted" aria-live="polite">
                Page {data.pagination.page} of {data.pagination.total_pages} · {data.pagination.total_items} total
              </span>
              <button
                type="button"
                className="button is-secondary"
                disabled={!data.pagination.has_next}
                onClick={() => setParam('page', (params.page ?? 1) + 1)}
              >
                Next
              </button>
            </div>
          ) : null}
        </>
      )}
    </section>
  )
}