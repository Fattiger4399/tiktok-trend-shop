import { useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  Select,
  Space,
  Typography,
  message,
} from 'antd'
import { useCreateMaterialRequest, useProduct } from '../hooks/queries'
import { prefillRequestBrief } from '../api/client'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { useI18n } from '../i18n/I18nProvider'

const USAGE_OPTIONS = ['listing', 'social', 'ad'] as const
const STYLE_OPTIONS = ['professional', 'playful', 'premium', 'friendly'] as const

interface RequestFormValues {
  usage: string
  style: string
  focus?: string
  notes?: string
}

export default function RequestNewPage() {
  const { t } = useI18n()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const productID = searchParams.get('product_id') ?? undefined
  const product = useProduct(productID)
  const createRequest = useCreateMaterialRequest()
  const [form] = Form.useForm<RequestFormValues>()
  const [prefilling, setPrefilling] = useState(false)
  const [messageApi, contextHolder] = message.useMessage()

  const handlePrefill = async () => {
    if (!productID) return
    setPrefilling(true)
    try {
      const { suggestion } = await prefillRequestBrief(productID)
      form.setFieldsValue({
        usage: suggestion.usage || undefined,
        style: suggestion.style || undefined,
        focus: suggestion.focus || undefined,
        notes: suggestion.notes || undefined,
      })
      messageApi.success(t('requests.new.prefillSuccess'))
    } catch (err) {
      messageApi.error((err as Error).message)
    } finally {
      setPrefilling(false)
    }
  }

  const handleSubmit = async (values: RequestFormValues) => {
    if (!productID) return
    try {
      const created = await createRequest.mutateAsync({
        product_id: productID,
        usage: values.usage,
        style: values.style,
        focus: values.focus ?? '',
        notes: values.notes ?? '',
      })
      messageApi.success(t('requests.new.submitSuccess'))
      navigate(`/requests/${created.id}`)
    } catch (err) {
      messageApi.error((err as Error).message)
    }
  }

  if (!productID) {
    return (
      <section>
        <Alert type="warning" showIcon message={t('requests.new.missingProduct')} />
      </section>
    )
  }
  if (product.isLoading) return <LoadingState label={t('common.loading')} rows={4} />
  if (product.error) return <ErrorState error={product.error as Error} />
  const detail = product.data
  if (!detail) return <EmptyState title={t('common.empty')} />

  const imageUrl = detail.detail?.image_url ?? null

  return (
    <section aria-labelledby="request-new-title">
      {contextHolder}
      <p>
        <Link to={`/products/${productID}`}>← {t('route.productDetail')}</Link>
      </p>
      <Typography.Title id="request-new-title" level={3} style={{ margin: '12px 0' }}>
        {t('requests.new.title')}
      </Typography.Title>

      <Card title={t('requests.new.product')} style={{ marginBottom: 16 }}>
        <Space align="start" size={16}>
          {imageUrl ? (
            <img
              src={imageUrl}
              alt={detail.product.title}
              style={{ width: 96, height: 96, objectFit: 'cover', borderRadius: 6 }}
            />
          ) : null}
          <Descriptions column={1} size="small">
            <Descriptions.Item label={t('product.title')}>{detail.product.title}</Descriptions.Item>
            <Descriptions.Item label={t('product.asin')}>
              {detail.product.asin ?? t('product.noAsin')}
            </Descriptions.Item>
            <Descriptions.Item label={t('product.marketplace')}>
              {detail.product.marketplace ?? (
                <span style={{ color: '#6b7280', fontStyle: 'italic' }}>{t('common.unavailable')}</span>
              )}
            </Descriptions.Item>
          </Descriptions>
        </Space>
      </Card>

      <Card>
        <Form<RequestFormValues>
          form={form}
          layout="vertical"
          style={{ maxWidth: 560 }}
          onFinish={handleSubmit}
        >
          <Form.Item
            name="usage"
            label={t('requests.new.usage')}
            rules={[{ required: true, message: t('requests.new.usageRequired') }]}
          >
            <Select
              placeholder={t('requests.new.usagePlaceholder')}
              options={USAGE_OPTIONS.map((value) => ({
                value,
                label: t(`requests.usage.${value}` as 'requests.usage.listing'),
              }))}
            />
          </Form.Item>
          <Form.Item
            name="style"
            label={t('requests.new.style')}
            rules={[{ required: true, message: t('requests.new.styleRequired') }]}
          >
            <Select
              placeholder={t('requests.new.stylePlaceholder')}
              options={STYLE_OPTIONS.map((value) => ({
                value,
                label: t(`requests.style.${value}` as 'requests.style.professional'),
              }))}
            />
          </Form.Item>
          <Form.Item name="focus" label={t('requests.new.focus')}>
            <Input placeholder={t('requests.new.focusPlaceholder')} />
          </Form.Item>
          <Form.Item name="notes" label={t('requests.new.notes')}>
            <Input.TextArea rows={4} placeholder={t('requests.new.notesPlaceholder')} />
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={createRequest.isPending}>
              {createRequest.isPending ? t('requests.new.submitting') : t('requests.new.submit')}
            </Button>
            <Button onClick={handlePrefill} loading={prefilling}>
              {prefilling ? t('requests.new.prefilling') : t('requests.new.prefill')}
            </Button>
          </Space>
        </Form>
      </Card>
    </section>
  )
}
