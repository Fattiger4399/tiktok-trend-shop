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
import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Empty,
  Row,
  Select,
  Space,
  Statistic,
  Tag,
  Typography,
  message,
} from 'antd'
import { useProduct, useProductMetrics, useCategories } from '../hooks/queries'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { assignProductCategory } from '../api/client'
import { useI18n } from '../i18n/I18nProvider'

export default function ProductDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { t, locale } = useI18n()
  const product = useProduct(id)
  const metrics = useProductMetrics(id, 'all')
  const categories = useCategories()
  const [selectedCategory, setSelectedCategory] = useState<string | undefined>(undefined)
  const [feedback, setFeedback] = useState<string | null>(null)
  const [messageApi, contextHolder] = message.useMessage()
  const dateLocale = locale === 'zh' ? 'zh-CN' : 'en-US'

  if (product.isLoading) return <LoadingState label={t('common.loading')} rows={6} />
  if (product.error) return <ErrorState error={product.error as Error} />
  const detail = product.data
  if (!detail) return <EmptyState title={t('common.empty')} />

  const handleAssign = async () => {
    if (!selectedCategory || !id) return
    try {
      await assignProductCategory(id, selectedCategory, 'workbench')
      messageApi.success(t('product.classification.assign') + ' ✓')
      setFeedback(t('product.classification.assign'))
      product.refetch()
    } catch (err) {
      messageApi.error((err as Error).message)
    }
  }

  const history = (metrics.data?.items ?? []).map((item) => ({
    captured_at: new Date(item.captured_at).toLocaleDateString(dateLocale),
    views: item.metrics.views ?? 0,
    sales: item.metrics.sales ?? 0,
    price: item.metrics.price ?? null,
    reviews: item.metrics.reviews ?? null,
  }))

  const priceSeries = history.filter((row) => row.price !== null)
  const completeness = detail.detail?.completeness === 'complete'
  const isEstimated = detail.latest_snapshot?.metric_kind === 'estimated'

  return (
    <section aria-labelledby="product-detail-title">
      {contextHolder}
      <p>
        <Link to="/trends">← {t('product.backToTrending')}</Link>
      </p>
      <Typography.Title id="product-detail-title" level={3} style={{ margin: '12px 0' }}>
        {detail.product.title}
      </Typography.Title>
      <Space size={8} wrap style={{ marginBottom: 16, color: '#6b7280' }}>
        <span>
          {detail.product.marketplace ? `${detail.product.marketplace} · ` : ''}
          {detail.product.asin ?? t('product.noAsin')}
        </span>
        <span>·</span>
        <span>
          {t('trends.table.updated')}{' '}
          {detail.latest_snapshot?.captured_at
            ? new Date(detail.latest_snapshot.captured_at).toLocaleString(dateLocale)
            : t('common.unavailable')}
        </span>
        {isEstimated ? <Tag color="orange">{t('product.estimatedTag')}</Tag> : null}
      </Space>

      <Row gutter={16}>
        <Col xs={24} md={14}>
          <Card
            title={t('product.title')}
            extra={
              completeness ? (
                <Tag color="green">{t('product.completeTag')}</Tag>
              ) : (
                <Tag color="orange">{t('product.incompleteTag')}</Tag>
              )
            }
          >
            {detail.detail ? (
              <Descriptions column={1} size="small" bordered>
                <Descriptions.Item label={t('product.price')}>
                  {detail.detail.price !== null && detail.detail.price !== undefined ? (
                    `${detail.detail.currency ?? ''} ${detail.detail.price.toFixed(2)}`.trim()
                  ) : (
                    <span style={{ color: '#6b7280', fontStyle: 'italic' }}>{t('common.unavailable')}</span>
                  )}
                </Descriptions.Item>
                <Descriptions.Item label={t('product.sellingPoints')}>
                  {detail.detail.selling_points?.length
                    ? detail.detail.selling_points.join(', ')
                    : <span style={{ color: '#6b7280', fontStyle: 'italic' }}>{t('common.unavailable')}</span>}
                </Descriptions.Item>
                <Descriptions.Item label={t('product.reviewSummary')}>
                  {detail.detail.review_summary ?? (
                    <span style={{ color: '#6b7280', fontStyle: 'italic' }}>{t('common.unavailable')}</span>
                  )}
                </Descriptions.Item>
                <Descriptions.Item label={t('product.shop')}>
                  {detail.detail.shop_name ?? (
                    <span style={{ color: '#6b7280', fontStyle: 'italic' }}>{t('common.unavailable')}</span>
                  )}
                  {detail.detail.brand ? ` · ${detail.detail.brand}` : ''}
                </Descriptions.Item>
                <Descriptions.Item label={t('product.missing')}>
                  {detail.detail.missing_fields?.length
                    ? detail.detail.missing_fields.join(', ')
                    : '—'}
                </Descriptions.Item>
              </Descriptions>
            ) : (
              <Empty description={t('product.noDetail')} />
            )}
          </Card>
        </Col>

        <Col xs={24} md={10}>
          <Card
            title={t('product.score.title')}
            extra={
              detail.score ? (
                <Tag color={
                  detail.score.confidence === 'high'
                    ? 'green'
                    : detail.score.confidence === 'medium'
                      ? 'orange'
                      : 'red'
                }>
                  {t(`confidence.${detail.score.confidence}` as 'confidence.high')}
                </Tag>
              ) : null
            }
          >
            {detail.score ? (
              <Space direction="vertical" style={{ width: '100%' }}>
                <Statistic
                  title={t('product.score.total')}
                  value={detail.score.total_score}
                  precision={2}
                  suffix={`${t('product.score.model')} ${detail.score.model_version} · ${t('product.score.window')} ${detail.score.time_window}`}
                />
                <Space wrap>
                  {Object.entries(detail.score.components).map(([key, value]) => (
                    <Tag key={key} bordered={false}>
                      <strong>{key}</strong>: {(value as number).toFixed(2)}
                    </Tag>
                  ))}
                </Space>
                {detail.score.missing_components?.length ? (
                  <Typography.Text type="secondary">
                    {t('product.score.missing')}: {detail.score.missing_components.join(', ')}
                  </Typography.Text>
                ) : null}
              </Space>
            ) : (
              <Empty description={t('product.score.noScore')} />
            )}
            <Typography.Title level={5} style={{ marginTop: 16 }}>
              {t('product.classification.title')}
            </Typography.Title>
            {detail.assignment ? (
              <p>
                <strong>{detail.assignment.canonical_category_id ?? '—'}</strong>{' '}
                <span style={{ color: '#6b7280' }}>
                  {t('product.classification.via')} {detail.assignment.method} (
                  {Math.round(detail.assignment.confidence * 100)}%)
                </span>
              </p>
            ) : (
              <Typography.Text type="secondary">{t('product.classification.none')}</Typography.Text>
            )}
            <Space.Compact style={{ width: '100%' }}>
              <Select
                style={{ width: '70%' }}
                value={selectedCategory}
                onChange={setSelectedCategory}
                placeholder={t('product.classification.choose')}
                options={(categories.data?.items ?? []).map((cat) => ({
                  value: cat.id,
                  label: cat.name,
                }))}
              />
              <Button
                type="primary"
                style={{ width: '30%' }}
                onClick={handleAssign}
                disabled={!selectedCategory}
              >
                {t('product.classification.assign')}
              </Button>
            </Space.Compact>
            {feedback ? (
              <Alert style={{ marginTop: 8 }} type="success" message={feedback} />
            ) : null}
          </Card>
        </Col>
      </Row>

      <Card
        title={t('product.history.title')}
        style={{ marginTop: 16 }}
        extra={
          history.length < 2 ? <Tag color="orange">{t('product.history.insufficientTag')}</Tag> : null
        }
      >
        {history.length === 0 ? (
          <Empty description={t('product.history.empty')} />
        ) : history.length < 2 ? (
          <Typography.Text type="secondary">{t('product.history.insufficient')}</Typography.Text>
        ) : (
          <Space direction="vertical" style={{ width: '100%' }} size={16}>
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
          </Space>
        )}
      </Card>
    </section>
  )
}