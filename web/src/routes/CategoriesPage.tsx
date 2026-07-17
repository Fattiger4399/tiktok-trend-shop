import { Link } from 'react-router-dom'
import { Card, Col, List, Row, Table, Tag, Typography } from 'antd'
import { useCategories, useMappings, useTrends } from '../hooks/queries'
import { EmptyState, LoadingState, ErrorState } from '../components/States'
import { useI18n } from '../i18n/I18nProvider'

export default function CategoriesPage() {
  const { t } = useI18n()
  const categories = useCategories()
  const mappings = useMappings()
  const unresolved = useTrends({ page_size: 50, sort: 'score' })

  const items = categories.data?.items ?? []
  const unresolvedItems = (unresolved.data?.items ?? []).filter((item) => !item.score)

  return (
    <section>
      <Typography.Title level={3} style={{ marginBottom: 16 }}>
        {t('categories.title')}
      </Typography.Title>
      <Row gutter={16}>
        <Col xs={24} md={12}>
          <Card title={t('categories.canonicalTitle')}>
            {categories.isLoading ? (
              <LoadingState label={t('categories.loading')} rows={4} />
            ) : categories.error ? (
              <ErrorState error={categories.error as Error} />
            ) : items.length === 0 ? (
              <EmptyState title={t('categories.empty.noCategories')} />
            ) : (
              <List
                size="small"
                dataSource={items}
                renderItem={(cat) => (
                  <List.Item>
                    <List.Item.Meta
                      title={<strong>{cat.name}</strong>}
                      description={
                        <>
                          <span style={{ color: '#6b7280' }}>/{cat.slug}</span>
                          {cat.parent_id ? (
                            <span style={{ color: '#6b7280' }}> · {cat.parent_id}</span>
                          ) : null}
                        </>
                      }
                    />
                  </List.Item>
                )}
              />
            )}
          </Card>
        </Col>

        <Col xs={24} md={12}>
          <Card title={t('categories.mappingsTitle')}>
            {mappings.isLoading ? (
              <LoadingState label={t('categories.mappingsLoading')} rows={4} />
            ) : mappings.error ? (
              <ErrorState error={mappings.error as Error} />
            ) : (mappings.data?.items ?? []).length === 0 ? (
              <EmptyState title={t('common.empty')} />
            ) : (
              <Table
                size="small"
                rowKey="id"
                pagination={false}
                dataSource={mappings.data?.items ?? []}
                columns={[
                  { title: t('categories.table.provider'), dataIndex: 'provider' },
                  { title: t('categories.table.source'), dataIndex: 'source_category_name', render: (v, r) => v ?? r.source_category_id },
                  { title: t('categories.table.canonical'), dataIndex: 'canonical_category_id' },
                  {
                    title: t('categories.table.confidence'),
                    dataIndex: 'confidence',
                    render: (v: number) => `${(v * 100).toFixed(0)}%`,
                  },
                ]}
              />
            )}
          </Card>
        </Col>
      </Row>

      <Card title={t('categories.reviewTitle')} style={{ marginTop: 16 }}>
        {unresolved.isLoading ? (
          <LoadingState label={t('categories.unresolvedLoading')} rows={3} />
        ) : unresolvedItems.length === 0 ? (
          <EmptyState
            title={t('categories.empty.noUnresolved')}
            description={t('categories.empty.noUnresolvedDescription')}
          />
        ) : (
          <Table
            size="small"
            rowKey="id"
            pagination={false}
            dataSource={unresolvedItems}
            columns={[
              {
                title: t('categories.table.product'),
                dataIndex: 'title',
                render: (value, record) => <Link to={`/products/${record.id}`}>{value}</Link>,
              },
              { title: t('categories.table.region'), dataIndex: 'region' },
              {
                title: t('categories.table.sourceCategory'),
                dataIndex: 'source_category',
                render: (v) =>
                  v ?? <Tag bordered={false}>{t('common.unavailable')}</Tag>,
              },
            ]}
          />
        )}
      </Card>
    </section>
  )
}