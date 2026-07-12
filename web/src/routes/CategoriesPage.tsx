import { Link } from 'react-router-dom'
import { useCategories, useMappings, useTrends } from '../hooks/queries'
import { EmptyState, LoadingState, ErrorState } from '../components/States'

export default function CategoriesPage() {
  const categories = useCategories()
  const mappings = useMappings()
  const unresolved = useTrends({ page_size: 50, sort: 'score' })

  const items = categories.data?.items ?? []
  const unresolvedItems = (unresolved.data?.items ?? []).filter((item) => !item.score)

  return (
    <section>
      <h2 className="card-title" style={{ marginBottom: 16 }}>Catalog &amp; Classification</h2>
      <div className="layout-two-column">
        <div className="card">
          <div className="card-header">
            <h3 className="card-title">Canonical categories</h3>
          </div>
          {categories.isLoading ? (
            <LoadingState label="Loading categories" rows={4} />
          ) : categories.error ? (
            <ErrorState error={categories.error as Error} />
          ) : items.length === 0 ? (
            <EmptyState title="No canonical categories seeded yet" />
          ) : (
            <ul className="category-list">
              {items.map((cat) => (
                <li key={cat.id}>
                  <strong>{cat.name}</strong>{' '}
                  <span className="muted">/{cat.slug}</span>
                  {cat.parent_id ? <span className="muted"> · parent {cat.parent_id}</span> : null}
                </li>
              ))}
            </ul>
          )}
        </div>

        <div className="card">
          <div className="card-header">
            <h3 className="card-title">Source mappings</h3>
          </div>
          {mappings.isLoading ? (
            <LoadingState label="Loading mappings" rows={4} />
          ) : mappings.error ? (
            <ErrorState error={mappings.error as Error} />
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Provider</th>
                  <th>Source</th>
                  <th>Canonical</th>
                  <th>Confidence</th>
                </tr>
              </thead>
              <tbody>
                {(mappings.data?.items ?? []).map((m) => (
                  <tr key={m.id}>
                    <td>{m.provider}</td>
                    <td>{m.source_category_name ?? m.source_category_id}</td>
                    <td>{m.canonical_category_id}</td>
                    <td>{(m.confidence * 100).toFixed(0)}%</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      <div className="card" style={{ marginTop: 16 }}>
        <div className="card-header">
          <h3 className="card-title">Needs review</h3>
        </div>
        {unresolved.isLoading ? (
          <LoadingState label="Looking up unresolved products" rows={3} />
        ) : unresolvedItems.length === 0 ? (
          <EmptyState
            title="No unresolved products"
            description="All imported products have a confident canonical assignment."
          />
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Product</th>
                <th>Region</th>
                <th>Source category</th>
              </tr>
            </thead>
            <tbody>
              {unresolvedItems.map((p) => (
                <tr key={p.id}>
                  <td><Link to={`/products/${p.id}`}>{p.title}</Link></td>
                  <td>{p.region}</td>
                  <td>{p.source_category ?? <span className="unavailable">unavailable</span>}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  )
}