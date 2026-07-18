import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Input,
  Modal,
  Space,
  Spin,
  Tag,
  Timeline,
  Typography,
  message,
} from 'antd'
import {
  useApproveRequest,
  useDeliverRequest,
  useGenerateRequestCopy,
  useProduct,
  useRejectRequest,
  useRequestReview,
  useRequestVariants,
} from '../hooks/queries'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { RequestStatusTag } from '../components/RequestStatusTag'
import { useAuth } from '../auth/AuthProvider'
import { useI18n } from '../i18n/I18nProvider'
import type { CopyVariant, MaterialRequest, ReviewEvent } from '../api/types'

const USAGE_VALUES = ['listing', 'social', 'ad']
const STYLE_VALUES = ['professional', 'playful', 'premium', 'friendly']

type TFunction = (key: string, params?: Record<string, string | number>) => string

function usageLabel(t: TFunction, value: string): string {
  if (!value) return '—'
  return USAGE_VALUES.includes(value)
    ? t(`requests.usage.${value}` as 'requests.usage.listing')
    : value
}

function styleLabel(t: TFunction, value: string): string {
  if (!value) return '—'
  return STYLE_VALUES.includes(value)
    ? t(`requests.style.${value}` as 'requests.style.professional')
    : value
}

function normalizeHashtags(value: unknown): string[] {
  if (Array.isArray(value)) return value.map(String)
  if (typeof value === 'string' && value.trim()) {
    try {
      const parsed: unknown = JSON.parse(value)
      if (Array.isArray(parsed)) return parsed.map(String)
    } catch {
      return [value]
    }
  }
  return []
}

function lastEvent(events: ReviewEvent[], action: ReviewEvent['action']): ReviewEvent | undefined {
  for (let i = events.length - 1; i >= 0; i -= 1) {
    if (events[i].action === action) return events[i]
  }
  return undefined
}

export default function RequestDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { t, locale } = useI18n()
  const { user } = useAuth()
  const review = useRequestReview(id)
  const variants = useRequestVariants(id)
  const product = useProduct(review.data?.request.product_id)
  const generate = useGenerateRequestCopy(id ?? '')
  const approve = useApproveRequest(id ?? '')
  const reject = useRejectRequest(id ?? '')
  const deliver = useDeliverRequest(id ?? '')
  const [rejectOpen, setRejectOpen] = useState(false)
  const [rejectReason, setRejectReason] = useState('')
  const [approveNotes, setApproveNotes] = useState<Record<string, string>>({})
  const [messageApi, contextHolder] = message.useMessage()
  const dateLocale = locale === 'zh' ? 'zh-CN' : 'en-US'
  // Review actions are signed with the logged-in username.
  const actor = user?.username ?? 'operator'

  if (review.isLoading) return <LoadingState label={t('common.loading')} rows={6} />
  if (review.error) return <ErrorState error={review.error as Error} onRetry={() => review.refetch()} />
  if (!review.data) return <EmptyState title={t('common.empty')} />

  const { request: req, events, delivery } = review.data
  const variantItems = variants.data?.items ?? []
  const rejectedEvent = lastEvent(events, 'rejected')
  const approvedEvent = lastEvent(events, 'approved')

  const handleGenerate = async () => {
    try {
      await generate.mutateAsync()
      messageApi.success(t('requests.detail.generateSuccess'))
    } catch (err) {
      messageApi.error((err as Error).message)
    }
  }

  const handleApprove = async (variantID: string) => {
    try {
      await approve.mutateAsync({
        variant_id: variantID,
        actor,
        note: approveNotes[variantID] ?? '',
      })
      messageApi.success(t('requests.detail.approveSuccess'))
    } catch (err) {
      messageApi.error((err as Error).message)
    }
  }

  const handleReject = async () => {
    if (!rejectReason.trim()) return
    try {
      await reject.mutateAsync({ reason: rejectReason.trim(), actor })
      messageApi.success(t('requests.detail.rejectSuccess'))
      setRejectOpen(false)
      setRejectReason('')
    } catch (err) {
      messageApi.error((err as Error).message)
    }
  }

  const handleDeliver = async () => {
    if (!approvedEvent?.variant_id) {
      messageApi.error(t('requests.detail.deliverMissingVariant'))
      return
    }
    try {
      await deliver.mutateAsync({ variant_id: approvedEvent.variant_id, actor })
      messageApi.success(t('requests.detail.deliverSuccess'))
    } catch (err) {
      messageApi.error((err as Error).message)
    }
  }

  return (
    <section aria-labelledby="request-detail-title">
      {contextHolder}
      <p>
        <Link to="/requests">← {t('requests.detail.backToList')}</Link>
      </p>
      <Typography.Title id="request-detail-title" level={3} style={{ margin: '12px 0' }}>
        {req.id}
      </Typography.Title>
      <Space size={8} wrap style={{ marginBottom: 16 }}>
        <RequestStatusTag status={req.status} />
        <span style={{ color: '#6b7280' }}>
          {t('requests.detail.created')} {new Date(req.created_at).toLocaleString(dateLocale)}
        </span>
      </Space>

      <Card title={t('requests.detail.productCard')} style={{ marginBottom: 16 }}>
        {product.data ? (
          <Descriptions column={1} size="small">
            <Descriptions.Item label={t('product.title')}>
              <Link to={`/products/${req.product_id}`}>{product.data.product.title}</Link>
            </Descriptions.Item>
            <Descriptions.Item label={t('product.asin')}>
              {product.data.product.asin ?? t('product.noAsin')}
            </Descriptions.Item>
            <Descriptions.Item label={t('product.marketplace')}>
              {product.data.product.marketplace ?? '—'}
            </Descriptions.Item>
          </Descriptions>
        ) : (
          <Link to={`/products/${req.product_id}`}>{req.product_id}</Link>
        )}
      </Card>

      <Card title={t('requests.detail.briefCard')} style={{ marginBottom: 16 }}>
        <Descriptions column={1} size="small" bordered>
          <Descriptions.Item label={t('requests.detail.usage')}>
            {usageLabel(t, req.usage)}
          </Descriptions.Item>
          <Descriptions.Item label={t('requests.detail.style')}>
            {styleLabel(t, req.style)}
          </Descriptions.Item>
          <Descriptions.Item label={t('requests.detail.focus')}>{req.focus || '—'}</Descriptions.Item>
          <Descriptions.Item label={t('requests.detail.notes')}>{req.notes || '—'}</Descriptions.Item>
          <Descriptions.Item label={t('requests.detail.updated')}>
            {new Date(req.updated_at).toLocaleString(dateLocale)}
          </Descriptions.Item>
        </Descriptions>
        {req.status === 'rejected' && rejectedEvent ? (
          <Alert
            style={{ marginTop: 12 }}
            type="error"
            showIcon
            message={t('requests.detail.lastRejectReason')}
            description={rejectedEvent.note}
          />
        ) : null}
      </Card>

      <ActionBar
        req={req}
        generating={generate.isPending}
        rejecting={reject.isPending}
        delivering={deliver.isPending}
        deliverDisabled={!approvedEvent?.variant_id}
        onGenerate={handleGenerate}
        onRejectOpen={() => setRejectOpen(true)}
        onDeliver={handleDeliver}
        t={t}
      />

      <Card title={t('requests.detail.variantsCard')} style={{ marginBottom: 16, marginTop: 16 }}>
        {variantItems.length === 0 ? (
          <Typography.Text type="secondary">{t('requests.detail.variantsEmpty')}</Typography.Text>
        ) : (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            {variantItems.map((variant) => (
              <VariantCard
                key={variant.id}
                variant={variant}
                approvable={req.status === 'generated'}
                approving={approve.isPending}
                note={approveNotes[variant.id] ?? ''}
                onNoteChange={(value) =>
                  setApproveNotes((prev) => ({ ...prev, [variant.id]: value }))
                }
                onApprove={() => handleApprove(variant.id)}
                t={t}
                dateLocale={dateLocale}
              />
            ))}
          </Space>
        )}
      </Card>

      <Card title={t('requests.detail.historyCard')} style={{ marginBottom: 16 }}>
        {events.length === 0 ? (
          <Typography.Text type="secondary">{t('requests.detail.historyEmpty')}</Typography.Text>
        ) : (
          <Timeline
            items={events.map((event) => ({
              color: event.action === 'approved' ? 'green' : 'red',
              children: (
                <div>
                  <Space size={8} wrap>
                    <Tag color={event.action === 'approved' ? 'green' : 'red'}>
                      {event.action === 'approved'
                        ? t('requests.detail.actionApproved')
                        : t('requests.detail.actionRejected')}
                    </Tag>
                    <span>
                      {t('requests.detail.actor')} {event.actor}
                    </span>
                    <span style={{ color: '#6b7280' }}>
                      {new Date(event.created_at).toLocaleString(dateLocale)}
                    </span>
                  </Space>
                  {event.note ? (
                    <div style={{ marginTop: 4 }}>
                      {t('requests.detail.note')}: {event.note}
                    </div>
                  ) : null}
                </div>
              ),
            }))}
          />
        )}
      </Card>

      {delivery ? (
        <Card title={t('requests.detail.deliveryCard')} style={{ marginBottom: 16 }}>
          <Descriptions column={1} size="small" bordered style={{ marginBottom: 12 }}>
            <Descriptions.Item label={t('deliveries.table.id')}>{delivery.id}</Descriptions.Item>
            <Descriptions.Item label={t('deliveries.table.actor')}>{delivery.actor}</Descriptions.Item>
            <Descriptions.Item label={t('deliveries.table.created')}>
              {new Date(delivery.created_at).toLocaleString(dateLocale)}
            </Descriptions.Item>
          </Descriptions>
          <Typography.Title level={5}>{t('deliveries.package')}</Typography.Title>
          <pre style={{ background: '#f5f6f8', padding: 12, borderRadius: 6, overflow: 'auto' }}>
            {JSON.stringify(delivery.package, null, 2)}
          </pre>
        </Card>
      ) : null}

      <Modal
        open={rejectOpen}
        title={t('requests.detail.reject')}
        okText={t('requests.detail.rejectConfirm')}
        cancelText={t('common.cancel')}
        okButtonProps={{ disabled: !rejectReason.trim(), loading: reject.isPending, danger: true }}
        onOk={handleReject}
        onCancel={() => setRejectOpen(false)}
      >
        <Typography.Paragraph>{t('requests.detail.rejectReason')}</Typography.Paragraph>
        <Input.TextArea
          rows={4}
          value={rejectReason}
          onChange={(e) => setRejectReason(e.target.value)}
          placeholder={t('requests.detail.rejectReasonRequired')}
        />
      </Modal>
    </section>
  )
}

interface ActionBarProps {
  req: MaterialRequest
  generating: boolean
  rejecting: boolean
  delivering: boolean
  deliverDisabled: boolean
  onGenerate: () => void
  onRejectOpen: () => void
  onDeliver: () => void
  t: TFunction
}

function ActionBar({
  req,
  generating,
  rejecting,
  delivering,
  deliverDisabled,
  onGenerate,
  onRejectOpen,
  onDeliver,
  t,
}: ActionBarProps) {
  // 单端模式：按钮只按 status 显示，不再按角色隐藏。
  // 恢复 RBAC 时按 user?.role === 'client' 隐藏 generate/deliver。
  if (req.status === 'submitted' || req.status === 'rejected') {
    return (
      <Card>
        <Button type="primary" onClick={onGenerate} loading={generating}>
          {t('requests.detail.generate')}
        </Button>
      </Card>
    )
  }
  if (req.status === 'generating') {
    return (
      <Card>
        <Space>
          <Spin size="small" />
          <span>{t('requests.detail.generating')}</span>
        </Space>
      </Card>
    )
  }
  if (req.status === 'generated') {
    return (
      <Card>
        <Button danger onClick={onRejectOpen} loading={rejecting}>
          {t('requests.detail.reject')}
        </Button>
      </Card>
    )
  }
  if (req.status === 'approved') {
    return (
      <Card>
        <Button type="primary" onClick={onDeliver} loading={delivering} disabled={deliverDisabled}>
          {t('requests.detail.deliver')}
        </Button>
      </Card>
    )
  }
  return null
}

interface VariantCardProps {
  variant: CopyVariant
  approvable: boolean
  approving: boolean
  note: string
  onNoteChange: (value: string) => void
  onApprove: () => void
  t: TFunction
  dateLocale: string
}

function VariantCard({
  variant,
  approvable,
  approving,
  note,
  onNoteChange,
  onApprove,
  t,
  dateLocale,
}: VariantCardProps) {
  const hashtags = normalizeHashtags(variant.hashtags)
  return (
    <Card
      type="inner"
      title={t('requests.detail.variant', { no: variant.variant_no })}
      extra={
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {t('requests.detail.provider')} {variant.provider} · {t('requests.detail.model')}{' '}
          {variant.model} · {t('requests.detail.promptVersion')} {variant.prompt_version}
        </Typography.Text>
      }
    >
      <Descriptions column={1} size="small" bordered>
        <Descriptions.Item label={t('requests.detail.hook')}>{variant.hook}</Descriptions.Item>
        <Descriptions.Item label={t('requests.detail.body')}>
          <span style={{ whiteSpace: 'pre-wrap' }}>{variant.body}</span>
        </Descriptions.Item>
        <Descriptions.Item label={t('requests.detail.caption')}>
          <span style={{ whiteSpace: 'pre-wrap' }}>{variant.caption}</span>
        </Descriptions.Item>
        <Descriptions.Item label={t('requests.detail.hashtags')}>
          {hashtags.length === 0
            ? '—'
            : hashtags.map((tag) => (
                <Tag key={tag} style={{ marginBottom: 4 }}>
                  {tag.startsWith('#') ? tag : `#${tag}`}
                </Tag>
              ))}
        </Descriptions.Item>
        <Descriptions.Item label={t('requests.detail.created')}>
          {new Date(variant.created_at).toLocaleString(dateLocale)}
        </Descriptions.Item>
      </Descriptions>
      {approvable ? (
        <Space.Compact style={{ width: '100%', marginTop: 12 }}>
          <Input
            value={note}
            onChange={(e) => onNoteChange(e.target.value)}
            placeholder={t('requests.detail.approveNotePlaceholder')}
          />
          <Button type="primary" onClick={onApprove} loading={approving}>
            {t('requests.detail.approve')}
          </Button>
        </Space.Compact>
      ) : null}
    </Card>
  )
}
