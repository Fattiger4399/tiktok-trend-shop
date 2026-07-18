import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Button, Card, Form, Input, Pagination, Select, Space, Table, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useCategories, useMarketplaces, useTrends } from '../hooks/queries'
import { useTrendsURL } from '../hooks/useTrendsURL'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { ProductRow } from '../components/ProductRow'
import { useI18n } from '../i18n/I18nProvider'
import type { ProductResult } from '../api/types'

export default function TrendingPage() {
  const { t, locale } = useI18n()
  const { params, setParam, clear } = useTrendsURL()
  const { data, error, isLoading, isFetching, refetch } = useTrends(params)
  const { data: marketplacesData } = useMarketplaces()
  const { data: categoriesData } = useCategories()
  const [searchInput, setSearchInput] = useState(params.q ?? '')

  const marketplaceOptions = useMemo(
    () => (marketplacesData?.items ?? []).map((m) => ({ value: m, label: m })),
    [marketplacesData],
  )
  const categoryOptions = useMemo(
    () =>
      (categoriesData?.items ?? [])
        .filter((c) => c.active)
        .map((c) => ({ value: c.id, label: c.name })),
    [categoriesData],
  )

  const products = data?.items ?? []
  const effective = (data?.effective ?? {}) as Record<string, unknown>
  const sortLabel = useMemo(() => {
    const value = (effective.sort as string) ?? params.sort ?? 'score'
    return t(`trends.sorts.${value}` as 'trends.sorts.score')
  }, [effective.sort, params.sort, t])

  const dateLocale = locale === 'zh' ? 'zh-CN' : 'en-US'

  const columns: ColumnsType<ProductResult> = useMemo(
    () => [
      {
        title: t('trends.table.product'),
        dataIndex: 'title',
        key: 'title',
        render: (_value, record) => (
          <div>
            <Link to={`/products/${record.id}`}>{record.title}</Link>
            <div style={{ color: '#6b7280', fontSize: 12 }}>
              {record.asin ?? t('product.noAsin')}
              {record.marketplace ? ` · ${record.marketplace}` : ''}
            </div>
          </div>
        ),
      },
      {
        title: t('trends.table.price'),
        key: 'price',
        width: 120,
        render: (_value, record) =>
          record.price !== undefined && record.price !== null ? (
            `${record.currency ?? ''} ${record.price.toFixed(2)}`.trim()
          ) : (
            <span style={{ color: '#6b7280', fontStyle: 'italic' }}>{t('common.unavailable')}</span>
          ),
      },
      {
        title: t('trends.table.score'),
        key: 'score',
        width: 160,
        render: (_value, record) =>
          record.score ? (
            <Space size={4}>
              <span>{record.score.total_score.toFixed(2)}</span>
              <Tag color={confidenceColor(record.score.confidence)}>
                {t(`confidence.${record.score.confidence}` as 'confidence.high')}
              </Tag>
            </Space>
          ) : (
            <span style={{ color: '#6b7280', fontStyle: 'italic' }}>{t('common.unavailable')}</span>
          ),
      },
      {
        title: t('trends.table.source'),
        key: 'source',
        width: 120,
        render: (_value, record) =>
          record.provenance?.provider ?? (
            <span style={{ color: '#6b7280', fontStyle: 'italic' }}>{t('common.unavailable')}</span>
          ),
      },
      {
        title: t('trends.table.updated'),
        key: 'updated',
        width: 200,
        render: (_value, record) => {
          if (!record.freshness?.last_captured_at) {
            return <span style={{ color: '#6b7280', fontStyle: 'italic' }}>{t('common.unavailable')}</span>
          }
          return (
            <Space direction="vertical" size={2}>
              <span>{new Date(record.freshness.last_captured_at).toLocaleString(dateLocale)}</span>
              {record.freshness.stale ? <Tag color="orange">{t('product.staleTag')}</Tag> : null}
            </Space>
          )
        },
      },
    ],
    [dateLocale, t],
  )

  return (
    <section aria-labelledby="trends-title">
      <Typography.Title id="trends-title" level={3} style={{ marginBottom: 16 }}>
        {t('trends.title')}
      </Typography.Title>
      <Card style={{ marginBottom: 16 }}>
        <Form layout="inline" style={{ rowGap: 8, columnGap: 8 }}>
          <Form.Item label={t('trends.filters.marketplace')}>
            <Select
              style={{ width: 160 }}
              value={params.marketplace ?? ''}
              onChange={(value) => setParam('marketplace', value)}
              options={[
                { value: '', label: t('trends.marketplaces.all') },
                ...marketplaceOptions,
              ]}
            />
          </Form.Item>
          <Form.Item label={t('trends.filters.category')}>
            <Select
              style={{ width: 200 }}
              value={params.category ?? ''}
              onChange={(value) => setParam('category', value)}
              options={[
                { value: '', label: t('trends.categories.all') },
                ...categoryOptions,
              ]}
            />
          </Form.Item>
          <Form.Item label={t('trends.filters.window')}>
            <Select
              style={{ width: 140 }}
              value={params.window ?? '30d'}
              onChange={(value) => setParam('window', value as typeof params.window)}
              options={[
                { value: '24h', label: t('trends.windows.24h') },
                { value: '7d', label: t('trends.windows.7d') },
                { value: '30d', label: t('trends.windows.30d') },
                { value: '90d', label: t('trends.windows.90d') },
                { value: 'all', label: t('trends.windows.all') },
              ]}
            />
          </Form.Item>
          <Form.Item label={t('trends.filters.sort')}>
            <Select
              style={{ width: 160 }}
              value={params.sort ?? 'score'}
              onChange={(value) => setParam('sort', value as typeof params.sort)}
              options={[
                { value: 'score', label: t('trends.sorts.score') },
                { value: 'updated', label: t('trends.sorts.updated') },
                { value: 'price', label: t('trends.sorts.price') },
                { value: 'created', label: t('trends.sorts.created') },
                { value: 'asin', label: t('trends.sorts.asin') },
              ]}
            />
          </Form.Item>
          <Form.Item label={t('trends.filters.direction')}>
            <Select
              style={{ width: 120 }}
              value={params.direction ?? 'desc'}
              onChange={(value) => setParam('direction', value as 'asc' | 'desc')}
              options={[
                { value: 'desc', label: t('trends.directions.desc') },
                { value: 'asc', label: t('trends.directions.asc') },
              ]}
            />
          </Form.Item>
          <Form.Item label={t('trends.filters.search')} style={{ flex: 1, minWidth: 200 }}>
            <Input.Search
              allowClear
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              onSearch={(value) => setParam('q', value)}
              placeholder={t('trends.filters.search')}
            />
          </Form.Item>
          <Form.Item>
            <Button onClick={clear}>{t('common.clear')}</Button>
          </Form.Item>
        </Form>
      </Card>

      {isLoading ? (
        <LoadingState label={t('common.loading')} rows={6} />
      ) : error ? (
        <ErrorState error={error as Error} onRetry={() => refetch()} />
      ) : products.length === 0 ? (
        <EmptyState title={t('trends.empty.title')} description={t('trends.empty.description')} />
      ) : (
        <>
          <Typography.Paragraph type="secondary" aria-live="polite">
            {t('trends.summary', { count: products.length, sort: sortLabel })}
            {isFetching ? ` · ${t('trends.refreshing')}` : ''}
          </Typography.Paragraph>
          <Table<ProductResult>
            rowKey="id"
            columns={columns}
            dataSource={products}
            pagination={false}
            size="middle"
            scroll={{ x: 720 }}
          />

          <div className="mobile-only" style={{ marginTop: 16 }}>
            {products.map((product) => (
              <div key={`mobile-${product.id}`} style={{ marginBottom: 12 }}>
                <ProductRow product={product} />
              </div>
            ))}
          </div>

          {data && data.pagination.total_pages > 1 ? (
            <Pagination
              style={{ marginTop: 16, textAlign: 'right' }}
              current={data.pagination.page}
              pageSize={data.pagination.page_size}
              total={data.pagination.total_items}
              showTotal={(total, range) =>
                `${range[0]}-${range[1]} ${t('common.of')} ${total} ${t('common.total')}`
              }
              onChange={(page) => setParam('page', page)}
            />
          ) : null}
        </>
      )}
    </section>
  )
}

function confidenceColor(confidence: string): string {
  if (confidence === 'high') return 'green'
  if (confidence === 'medium') return 'orange'
  return 'red'
}