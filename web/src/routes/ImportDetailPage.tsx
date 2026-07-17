import { Link, useParams } from 'react-router-dom'
import { Card, Col, Descriptions, Row, Table, Tag, Typography } from 'antd'
import { useImportDetail } from '../hooks/queries'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { useI18n } from '../i18n/I18nProvider'

export default function ImportDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { t } = useI18n()
  const { data, isLoading, error } = useImportDetail(id)
  if (isLoading) return <LoadingState label={t('common.loading')} rows={3} />
  if (error) return <ErrorState error={error as Error} />
  if (!data) return <EmptyState title={t('common.empty')} />

  const { job, results } = data
  return (
    <section>
      <p>
        <Link to="/imports">← {t('nav.imports')}</Link>
      </p>
      <Typography.Title level={3} style={{ margin: '12px 0' }}>
        {job.id}
      </Typography.Title>
      <Typography.Paragraph type="secondary">
        {job.filename ?? '—'} ·{' '}
        <Tag color={job.status === 'completed' ? 'green' : job.status === 'failed' ? 'red' : 'orange'}>
          {t(`imports.status.${job.status}` as 'imports.status.completed')}
        </Tag>
      </Typography.Paragraph>
      <Row gutter={16}>
        <Col xs={24} md={10}>
          <Card title={t('imports.detail.summary')}>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label={t('imports.detail.imported')}>{job.imported_rows}</Descriptions.Item>
              <Descriptions.Item label={t('imports.detail.updated')}>{job.updated_rows}</Descriptions.Item>
              <Descriptions.Item label={t('imports.detail.duplicates')}>{job.duplicate_rows}</Descriptions.Item>
              <Descriptions.Item label={t('imports.detail.rejected')}>{job.rejected_rows}</Descriptions.Item>
              <Descriptions.Item label={t('imports.detail.startedAt')}>
                {new Date(job.started_at).toLocaleString()}
              </Descriptions.Item>
              <Descriptions.Item label={t('imports.detail.completedAt')}>
                {job.completed_at ? new Date(job.completed_at).toLocaleString() : '—'}
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>
        <Col xs={24} md={14}>
          <Card title={t('imports.detail.outcomes')}>
            {results.length === 0 ? (
              <Typography.Text type="secondary">{t('imports.detail.noOutcomes')}</Typography.Text>
            ) : (
              <Table
                size="small"
                rowKey={(r) => `${r.row_number}-${r.status}`}
                pagination={{ pageSize: 20 }}
                dataSource={results}
                columns={[
                  { title: t('imports.detail.row'), dataIndex: 'row_number', width: 80 },
                  {
                    title: t('imports.table.status'),
                    dataIndex: 'status',
                    render: (s: string) => <Tag>{s}</Tag>,
                  },
                  { title: t('imports.detail.reason'), dataIndex: 'reason' },
                ]}
              />
            )}
          </Card>
        </Col>
      </Row>
    </section>
  )
}