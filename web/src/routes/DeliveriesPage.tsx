import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Descriptions, Pagination, Table, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useDeliveries } from '../hooks/queries'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { useI18n } from '../i18n/I18nProvider'
import type { Delivery } from '../api/types'

const PAGE_SIZE = 20

export default function DeliveriesPage() {
  const { t, locale } = useI18n()
  const [page, setPage] = useState(1)
  const params = useMemo(() => ({ page, page_size: PAGE_SIZE }), [page])
  const { data, error, isLoading, refetch } = useDeliveries(params)
  const dateLocale = locale === 'zh' ? 'zh-CN' : 'en-US'

  const columns: ColumnsType<Delivery> = useMemo(
    () => [
      {
        title: t('deliveries.table.id'),
        dataIndex: 'id',
        key: 'id',
        render: (value: string, record) => (
          <Link to={`/requests/${record.request_id}`}>{value}</Link>
        ),
      },
      {
        title: t('deliveries.table.product'),
        key: 'product',
        render: (_value, record) => {
          const product = record.package?.product
          if (!product) return record.package?.request?.product_id ?? '—'
          return (
            <div>
              <Link to={`/products/${product.id}`}>{product.title}</Link>
              <div style={{ color: '#6b7280', fontSize: 12 }}>
                {product.asin ?? t('product.noAsin')}
                {product.marketplace ? ` · ${product.marketplace}` : ''}
              </div>
            </div>
          )
        },
      },
      {
        title: t('deliveries.table.variant'),
        key: 'variant',
        width: 120,
        render: (_value, record) =>
          t('deliveries.variantNo', { no: record.package?.variant?.variant_no ?? '—' }),
      },
      {
        title: t('deliveries.table.actor'),
        dataIndex: 'actor',
        key: 'actor',
        width: 120,
      },
      {
        title: t('deliveries.table.created'),
        dataIndex: 'created_at',
        key: 'created_at',
        width: 200,
        render: (value: string) => new Date(value).toLocaleString(dateLocale),
      },
    ],
    [dateLocale, t],
  )

  const items = data?.items ?? []

  return (
    <section aria-labelledby="deliveries-title">
      <Typography.Title id="deliveries-title" level={3} style={{ marginBottom: 16 }}>
        {t('deliveries.title')}
      </Typography.Title>

      {isLoading ? (
        <LoadingState label={t('common.loading')} rows={6} />
      ) : error ? (
        <ErrorState error={error as Error} onRetry={() => refetch()} />
      ) : items.length === 0 ? (
        <EmptyState
          title={t('deliveries.empty.title')}
          description={t('deliveries.empty.description')}
        />
      ) : (
        <>
          <Table<Delivery>
            rowKey="id"
            columns={columns}
            dataSource={items}
            pagination={false}
            size="middle"
            scroll={{ x: 720 }}
            expandable={{
              expandedRowRender: (record) => (
                <div>
                  <Descriptions column={1} size="small" style={{ marginBottom: 12 }}>
                    <Descriptions.Item label={t('requests.detail.hook')}>
                      {record.package?.variant?.hook ?? '—'}
                    </Descriptions.Item>
                    <Descriptions.Item label={t('requests.detail.caption')}>
                      {record.package?.variant?.caption ?? '—'}
                    </Descriptions.Item>
                  </Descriptions>
                  <Typography.Title level={5}>{t('deliveries.package')}</Typography.Title>
                  <pre style={{ background: '#f5f6f8', padding: 12, borderRadius: 6, overflow: 'auto' }}>
                    {JSON.stringify(record.package, null, 2)}
                  </pre>
                </div>
              ),
            }}
          />
          {data && data.pagination.total_pages > 1 ? (
            <Pagination
              style={{ marginTop: 16, textAlign: 'right' }}
              current={data.pagination.page}
              pageSize={data.pagination.page_size}
              total={data.pagination.total_items}
              showTotal={(total, range) =>
                `${range[0]}-${range[1]} ${t('common.of')} ${total} ${t('common.total')}`
              }
              onChange={(next) => setPage(next)}
            />
          ) : null}
        </>
      )}
    </section>
  )
}
