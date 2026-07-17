import { useState } from 'react'
import { Link } from 'react-router-dom'
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd'
import { InboxOutlined } from '@ant-design/icons'
import { useImports } from '../hooks/queries'
import { createImport, type CreateImportResult } from '../api/client'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { useI18n } from '../i18n/I18nProvider'

const MAX_BYTES = 10 * 1024 * 1024

export default function ImportsPage() {
  const { t } = useI18n()
  const imports = useImports(50)
  const [file, setFile] = useState<File | null>(null)
  const [region, setRegion] = useState<string>('CN')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [messageApi, contextHolder] = message.useMessage()

  const submit = async () => {
    if (!file) {
      setError(t('imports.error'))
      return
    }
    if (file.size > MAX_BYTES) {
      setError(t('imports.tooLarge', { size: (MAX_BYTES / 1024 / 1024).toFixed(0) }))
      return
    }
    setError(null)
    setSubmitting(true)
    try {
      const result = await createImport({ file, region })
      messageApi.success(t('imports.success', { id: result.job_id, count: result.imported_rows }))
      setFile(null)
      imports.refetch()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  const columns = [
    { title: t('imports.table.file'), dataIndex: 'filename', render: (v: string | null) => v ?? <Tag>{t('common.unavailable')}</Tag> },
    {
      title: t('imports.table.status'),
      dataIndex: 'status',
      render: (status: string) => (
        <Tag color={status === 'completed' ? 'green' : status === 'failed' ? 'red' : 'orange'}>
          {t(`imports.status.${status}` as 'imports.status.completed')}
        </Tag>
      ),
    },
    { title: t('imports.table.imported'), dataIndex: 'imported_rows' },
    { title: t('imports.table.updated'), dataIndex: 'updated_rows' },
    { title: t('imports.table.rejected'), dataIndex: 'rejected_rows' },
    {
      title: t('imports.table.started'),
      dataIndex: 'started_at',
      render: (v: string) => new Date(v).toLocaleString(),
    },
    {
      title: t('common.actions'),
      key: 'actions',
      render: (_: unknown, record: { id: string }) => <Link to={`/imports/${record.id}`}>{t('common.view')}</Link>,
    },
  ]

  return (
    <section>
      {contextHolder}
      <Typography.Title level={3} style={{ marginBottom: 16 }}>
        {t('imports.title')}
      </Typography.Title>
      <Card title={t('imports.uploadTitle')}>
        <Form layout="vertical" onFinish={submit}>
          <Form.Item label={t('imports.file')} required>
            <Upload.Dragger
              multiple={false}
              beforeUpload={(uploadFile) => {
                if (uploadFile.size > MAX_BYTES) {
                  setError(t('imports.tooLarge', { size: (MAX_BYTES / 1024 / 1024).toFixed(0) }))
                  return Upload.LIST_IGNORE
                }
                setFile(uploadFile)
                setError(null)
                return false
              }}
              fileList={file ? [{ uid: '1', name: file.name, status: 'done' as const }] : []}
              onRemove={() => setFile(null)}
              accept=".csv,text/csv"
            >
              <p className="ant-upload-drag-icon">
                <InboxOutlined />
              </p>
              <p className="ant-upload-text">{file ? file.name : t('imports.file')}</p>
              <p className="ant-upload-hint">{t('imports.help')}</p>
            </Upload.Dragger>
          </Form.Item>
          <Space>
            <Form.Item label={t('imports.region')}>
              <Select
                style={{ width: 120 }}
                value={region}
                onChange={setRegion}
                options={['CN', 'US', 'UK', 'JP', 'DE'].map((r) => ({ value: r, label: r }))}
              />
            </Form.Item>
            <Form.Item label=" " colon={false}>
              <Button type="primary" htmlType="submit" loading={submitting}>
                {submitting ? t('imports.submitting') : t('imports.submit')}
              </Button>
            </Form.Item>
          </Space>
          {error ? <Alert type="error" message={error} showIcon style={{ marginBottom: 12 }} /> : null}
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {t('imports.help')}
          </Typography.Paragraph>
        </Form>
      </Card>

      <Card title={t('imports.historyTitle')} style={{ marginTop: 16 }}>
        {imports.isLoading ? (
          <LoadingState label={t('imports.loading')} rows={3} />
        ) : imports.error ? (
          <ErrorState error={imports.error as Error} />
        ) : (imports.data?.items ?? []).length === 0 ? (
          <EmptyState title={t('imports.empty')} description={t('imports.emptyDescription')} />
        ) : (
          <Table
            size="middle"
            rowKey="id"
            pagination={{ pageSize: 10 }}
            dataSource={imports.data?.items ?? []}
            columns={columns}
          />
        )}
      </Card>
    </section>
  )
}