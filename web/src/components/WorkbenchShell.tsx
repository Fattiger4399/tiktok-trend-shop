import { useMemo } from 'react'
import { Layout, Menu, Typography, theme as antdTheme } from 'antd'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { TrendingUp, Tags, Upload } from 'lucide-react'
import { useI18n } from '../i18n/I18nProvider'
import { LanguageSwitcher } from './LanguageSwitcher'

const { Sider, Header, Content } = Layout

export default function WorkbenchShell() {
  const location = useLocation()
  const navigate = useNavigate()
  const { t } = useI18n()
  const { token } = antdTheme.useToken()

  const selectedKey = useMemo(() => {
    const path = location.pathname
    if (path.startsWith('/trends')) return 'trends'
    if (path.startsWith('/categories')) return 'categories'
    if (path.startsWith('/imports')) return 'imports'
    return 'trends'
  }, [location.pathname])

  const title = useMemo(() => {
    const path = location.pathname
    if (path.startsWith('/products/')) return t('route.productDetail')
    if (path.startsWith('/imports/')) return t('route.importDetail')
    return t(`route.${selectedKey}` as 'route.trends')
  }, [location.pathname, selectedKey, t])

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        breakpoint="lg"
        collapsedWidth="0"
        width={232}
        style={{ background: token.colorBgContainer === '#ffffff' ? '#131a2b' : undefined }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            padding: '20px 16px',
            color: '#f5f7fa',
          }}
        >
          <span
            style={{
              width: 32,
              height: 32,
              borderRadius: 6,
              background: token.colorPrimary,
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#fff',
              fontWeight: 700,
              fontSize: 13,
            }}
          >
            TTS
          </span>
          <span style={{ fontWeight: 600, fontSize: 14 }}>{t('app.brand')}</span>
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          onClick={(info) => navigate(`/${info.key}`)}
          items={[
            { key: 'trends', icon: <TrendingUp size={16} aria-hidden />, label: t('nav.trending') },
            { key: 'categories', icon: <Tags size={16} aria-hidden />, label: t('nav.categories') },
            { key: 'imports', icon: <Upload size={16} aria-hidden />, label: t('nav.imports') },
          ]}
          style={{ background: 'transparent', borderInlineEnd: 'none' }}
        />
      </Sider>
      <Layout>
        <Header
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            background: token.colorBgContainer,
            borderBottom: `1px solid ${token.colorBorderSecondary}`,
            paddingInline: 32,
          }}
        >
          <Typography.Title level={4} style={{ margin: 0 }}>
            {title}
          </Typography.Title>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <span style={{ color: token.colorTextSecondary, fontSize: 12 }}>{t('app.environment')}</span>
            <LanguageSwitcher />
          </div>
        </Header>
        <Content style={{ padding: 24, background: '#f5f6f8' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}

// Keep the Link export available for components that need it.
export { Link }