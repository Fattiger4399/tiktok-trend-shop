import { Link } from 'react-router-dom'
import type { ProductResult } from '../api/types'

export interface ProductRowProps {
  product: ProductResult
}

export function ProductRow({ product }: ProductRowProps) {
  const price = formatPrice(product.price, product.currency)
  const freshness = product.freshness?.last_captured_at
    ? new Date(product.freshness.last_captured_at).toLocaleString()
    : 'no observation'
  return (
    <article className="product-row-card" data-testid="product-row">
      <div className="product-row-thumb" aria-hidden>
        {product.image_url ? (
          <img src={product.image_url} alt="" loading="lazy" />
        ) : (
          <span>No image</span>
        )}
      </div>
      <div>
        <h3 className="card-title">
          <Link to={`/products/${product.id}`}>{product.title}</Link>
        </h3>
        <p className="muted">
          {product.marketplace ? `${product.marketplace} · ` : ''}
          {product.asin ?? 'no ASIN'}
          {' · '}
          {product.provenance?.provider ?? 'unknown source'}
        </p>
        <p>
          <strong>{price ?? 'Price unavailable'}</strong>
          {' · '}
          Score:{' '}
          {product.score ? product.score.total_score.toFixed(2) : 'unavailable'}
          {product.score ? (
            <span className={`tag ${confidenceClass(product.score.confidence)}`} style={{ marginLeft: 8 }}>
              {product.score.confidence} confidence
            </span>
          ) : null}
        </p>
        <p className="muted">
          Last captured {freshness}
          {product.freshness?.stale ? <span className="tag is-warning" style={{ marginLeft: 8 }}>Stale</span> : null}
        </p>
      </div>
    </article>
  )
}

function formatPrice(price: number | null | undefined, currency: string | null | undefined): string | null {
  if (price === null || price === undefined) return null
  return `${currency ?? ''} ${price.toFixed(2)}`.trim()
}

function confidenceClass(confidence: string): string {
  if (confidence === 'high') return 'is-positive'
  if (confidence === 'medium') return 'is-warning'
  return 'is-negative'
}