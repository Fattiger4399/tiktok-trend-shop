import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Card, Form, Pagination, Select, Table, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useMaterialRequests, useProduct } from '../hooks/queries'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { RequestStatusTag } from '../components/RequestStatusTag'
import { useI18n } from '../i18n/I18nProvider'
import type { MaterialRequest } from '../api/types'

const PAGE_SIZE = 20
const STATUSES = ['submitted', 'generating', 'generated', 'approved', 'rejected', 'delivered']
const USAGE_VALUES = ['listing', 'social', 'ad']

type TFunction = (key: string, params?: Record<string, string | number>) => string

function usageLabel(t: TFunction, value: string): string {
  return USAGE_VALUES.includes(value)
    ? t(`requests.usage.${value}` as 'requests.usage.listing')
    : value
}

function ProductTitleCell({ productID }: { productID: string }) {
  const product = useProduct(productID)
  const title = product.data?.product.title ?? productID
  return <Link to={`/products/${productID}`}>{title}</Link>
}

export default function RequestsPage() {
  const { t, locale } = useI18n()
  const [status, setStatus] = useState('')
  const [page, setPage] = useState(1)
  const params = useMemo(
    () => ({ status: status || undefined, page, page_size: PAGE_SIZE }),
    [status, page],
  )
  const { data, error, isLoading, refetch } = useMaterialRequests(params)
  const dateLocale = locale === 'zh' ? 'zh-CN' : 'en-US'

  const columns: ColumnsType<MaterialRequest> = useMemo(
    () => [
      {
        title: t('requests.table.id'),
        dataIndex: 'id',
        key: 'id',
        render: (value: string) => <Link to={`/requests/${value}`}>{value}</Link>,
      },
      {
        title: t('requests.table.product'),
        dataIndex: 'product_id',
        key: 'product_id',
        render: (value: string) => <ProductTitleCell productID={value} />,
      },
      {
        title: t('requests.table.usage'),
        dataIndex: 'usage',
        key: 'usage',
        width: 140,
        render: (value: string) => (value ? usageLabel(t, value) : '—'),
      },
      {
        title: t('requests.table.status'),
        dataIndex: 'status',
        key: 'status',
        width: 120,
        render: (value: string) => <RequestStatusTag status={value} />,
      },
      {
        title: t('requests.table.created'),
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
    <section aria-labelledby="requests-title">
      <Typography.Title id="requests-title" level={3} style={{ marginBottom: 16 }}>
        {t('requests.title')}
      </Typography.Title>
      <Card style={{ marginBottom: 16 }}>
        <Form layout="inline" style={{ rowGap: 8, columnGap: 8 }}>
          <Form.Item label={t('requests.filters.status')}>
            <Select
              style={{ width: 160 }}
              value={status}
              onChange={(value) => {
                setStatus(value)
                setPage(1)
              }}
              options={[
                { value: '', label: t('requests.filters.allStatuses') },
                ...STATUSES.map((s) => ({
                  value: s,
                  label: t(`requests.status.${s}` as 'requests.status.submitted'),
                })),
              ]}
            />
          </Form.Item>
        </Form>
      </Card>

      {isLoading ? (
        <LoadingState label={t('common.loading')} rows={6} />
      ) : error ? (
        <ErrorState error={error as Error} onRetry={() => refetch()} />
      ) : items.length === 0 ? (
        <EmptyState title={t('requests.empty.title')} description={t('requests.empty.description')} />
      ) : (
        <>
          <Table<MaterialRequest>
            rowKey="id"
            columns={columns}
            dataSource={items}
            pagination={false}
            size="middle"
            scroll={{ x: 720 }}
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
