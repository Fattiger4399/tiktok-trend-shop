import { Card, Space, Tag } from 'antd'
import { Link } from 'react-router-dom'
import type { ProductResult } from '../api/types'
import { useI18n } from '../i18n/I18nProvider'

export interface ProductRowProps {
  product: ProductResult
}

export function ProductRow({ product }: ProductRowProps) {
  const { t, locale } = useI18n()
  const price = formatPrice(product.price, product.currency)
  const freshness = product.freshness?.last_captured_at
    ? new Date(product.freshness.last_captured_at).toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US')
    : t('common_columns.noObservation')

  return (
    <Card size="small" bodyStyle={{ padding: 12 }}>
      <div style={{ display: 'grid', gridTemplateColumns: '64px 1fr', gap: 12 }}>
        <div
          aria-hidden
          style={{
            width: 64,
            height: 64,
            borderRadius: 6,
            background: '#f0f2f5',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#6b7280',
            fontSize: 12,
            textAlign: 'center',
            overflow: 'hidden',
          }}
        >
          {product.image_url ? (
            <img src={product.image_url} alt="" loading="lazy" style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
          ) : (
            <span>{t('common_columns.noImage')}</span>
          )}
        </div>
        <div>
          <h4 style={{ margin: 0, fontSize: 14 }}>
            <Link to={`/products/${product.id}`}>{product.title}</Link>
          </h4>
          <Space size={4} style={{ color: '#6b7280', fontSize: 12 }}>
            <span>{product.marketplace ?? '—'}</span>
            <span>·</span>
            <span>{product.asin ?? t('product.noAsin')}</span>
            <span>·</span>
            <span>{product.provenance?.provider ?? t('common_columns.noSource')}</span>
          </Space>
          <div style={{ marginTop: 4 }}>
            <strong>{price ?? t('product.price') + ': ' + t('common.unavailable')}</strong>
            <span style={{ marginLeft: 8 }}>{t('product.score.title')}:</span>
            <span style={{ marginLeft: 4 }}>
              {product.score ? product.score.total_score.toFixed(2) : t('common.unavailable')}
            </span>
            {product.score ? (
              <Tag
                color={
                  product.score.confidence === 'high'
                    ? 'green'
                    : product.score.confidence === 'medium'
                      ? 'orange'
                      : 'red'
                }
                style={{ marginLeft: 8 }}
              >
                {t(`confidence.${product.score.confidence}` as 'confidence.high')}
              </Tag>
            ) : null}
          </div>
          <div style={{ color: '#6b7280', fontSize: 12, marginTop: 4 }}>
            {t('common_columns.captured')} {freshness}
            {product.freshness?.stale ? (
              <Tag color="orange" style={{ marginLeft: 8 }}>{t('product.staleTag')}</Tag>
            ) : null}
          </div>
        </div>
      </div>
    </Card>
  )
}

function formatPrice(price: number | null | undefined, currency: string | null | undefined): string | null {
  if (price === null || price === undefined) return null
  return `${currency ?? ''} ${price.toFixed(2)}`.trim()
}