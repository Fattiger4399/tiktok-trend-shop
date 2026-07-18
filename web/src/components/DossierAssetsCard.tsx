import { useState } from 'react'
import { Button, Card, Empty, Image, Input, List, Select, Space, Tag, Typography, message } from 'antd'
import { useAddDossierAsset, useDeleteDossierAsset, useDossierAssets } from '../hooks/queries'
import { ErrorState, LoadingState } from './States'
import { useI18n } from '../i18n/I18nProvider'
import type { DossierAsset, DossierAssetKind } from '../api/types'

const KINDS: DossierAssetKind[] = ['image', 'text', 'link']

function AssetBody({ asset }: { asset: DossierAsset }) {
  if (asset.kind === 'image') {
    return <Image src={asset.url} alt={asset.note || asset.source} width={120} />
  }
  if (asset.kind === 'link') {
    return (
      <Typography.Link href={asset.url} target="_blank" rel="noreferrer">
        {asset.url}
      </Typography.Link>
    )
  }
  return (
    <Typography.Paragraph
      style={{
        margin: 0,
        paddingLeft: 12,
        borderLeft: '3px solid #d1d5db',
        color: '#374151',
        whiteSpace: 'pre-wrap',
      }}
    >
      {asset.content}
    </Typography.Paragraph>
  )
}

export function DossierAssetsCard({ productID }: { productID: string }) {
  const { t, locale } = useI18n()
  const assets = useDossierAssets(productID)
  const addAsset = useAddDossierAsset(productID)
  const deleteAsset = useDeleteDossierAsset(productID)
  const [messageApi, contextHolder] = message.useMessage()
  const [kind, setKind] = useState<DossierAssetKind>('image')
  const [url, setUrl] = useState('')
  const [content, setContent] = useState('')
  const [source, setSource] = useState('')
  const [note, setNote] = useState('')
  const dateLocale = locale === 'zh' ? 'zh-CN' : 'en-US'

  const handleAdd = async () => {
    const trimmedURL = url.trim()
    const trimmedContent = content.trim()
    if ((kind === 'image' || kind === 'link') && !trimmedURL) {
      messageApi.error(t('product.dossier.needUrl'))
      return
    }
    if (kind === 'text' && !trimmedContent) {
      messageApi.error(t('product.dossier.needContent'))
      return
    }
    try {
      await addAsset.mutateAsync({
        kind,
        url: trimmedURL,
        content: trimmedContent,
        source: source.trim(),
        note: note.trim(),
      })
      messageApi.success(t('product.dossier.added'))
      setUrl('')
      setContent('')
      setSource('')
      setNote('')
    } catch (err) {
      messageApi.error((err as Error).message)
    }
  }

  const handleDelete = async (assetID: string) => {
    try {
      await deleteAsset.mutateAsync(assetID)
      messageApi.success(t('product.dossier.deleted'))
    } catch (err) {
      messageApi.error((err as Error).message)
    }
  }

  return (
    <Card title={t('product.dossier.title')} style={{ marginTop: 16 }}>
      {contextHolder}
      <Space direction="vertical" size={8} style={{ width: '100%', marginBottom: 16 }}>
        <Space size={8} wrap>
          <Select
            aria-label={t('product.dossier.kind')}
            style={{ width: 120 }}
            value={kind}
            onChange={setKind}
            options={KINDS.map((k) => ({ value: k, label: t(`product.dossier.kinds.${k}`) }))}
          />
          {kind !== 'text' ? (
            <Input
              aria-label={t('product.dossier.url')}
              style={{ width: 320 }}
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder={t('product.dossier.urlPlaceholder')}
            />
          ) : null}
          <Input
            aria-label={t('product.dossier.source')}
            style={{ width: 200 }}
            value={source}
            onChange={(e) => setSource(e.target.value)}
            placeholder={t('product.dossier.sourcePlaceholder')}
          />
          <Input
            aria-label={t('product.dossier.note')}
            style={{ width: 200 }}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder={t('product.dossier.notePlaceholder')}
          />
        </Space>
        {kind === 'text' ? (
          <Input.TextArea
            aria-label={t('product.dossier.content')}
            rows={2}
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder={t('product.dossier.contentPlaceholder')}
          />
        ) : null}
        <Button type="primary" onClick={handleAdd} loading={addAsset.isPending}>
          {t('product.dossier.add')}
        </Button>
      </Space>
      {assets.isLoading ? (
        <LoadingState label={t('common.loading')} rows={2} />
      ) : assets.error ? (
        <ErrorState error={assets.error as Error} />
      ) : (assets.data?.items ?? []).length === 0 ? (
        <Empty description={t('product.dossier.empty')} />
      ) : (
        <List
          size="small"
          dataSource={assets.data?.items ?? []}
          renderItem={(item) => (
            <List.Item
              actions={[
                <Button
                  key="delete"
                  type="text"
                  size="small"
                  danger
                  onClick={() => handleDelete(item.id)}
                >
                  {t('product.dossier.delete')}
                </Button>,
              ]}
            >
              <Space direction="vertical" size={4} style={{ width: '100%' }}>
                <AssetBody asset={item} />
                <Space size={8} wrap style={{ color: '#6b7280', fontSize: 12 }}>
                  <Tag bordered={false}>{t(`product.dossier.kinds.${item.kind}`)}</Tag>
                  {item.source ? <span>{item.source}</span> : null}
                  {item.note ? <span>· {item.note}</span> : null}
                  <span>
                    · {t('product.dossier.by')} {item.created_by || '—'} ·{' '}
                    {new Date(item.created_at).toLocaleString(dateLocale)}
                  </span>
                </Space>
                {item.kind !== 'text' && item.content ? (
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {item.content}
                  </Typography.Text>
                ) : null}
              </Space>
            </List.Item>
          )}
        />
      )}
    </Card>
  )
}
