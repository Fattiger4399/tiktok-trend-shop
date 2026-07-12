import { useMemo } from 'react'
import { Outlet, NavLink, useLocation } from 'react-router-dom'
import { TrendingUp, Tags, Upload, Menu } from 'lucide-react'
import './WorkbenchShell.css'

const TITLES: Record<string, string> = {
  '/trends': 'Trending Products',
  '/categories': 'Catalog & Classification',
  '/imports': 'CSV Imports',
}

export default function WorkbenchShell() {
  const location = useLocation()
  const title = useMemo(() => {
    const path = location.pathname.startsWith('/products/')
      ? 'Product Detail'
      : location.pathname.startsWith('/imports/')
        ? 'Import Detail'
        : TITLES[location.pathname] ?? 'Workbench'
    return path
  }, [location.pathname])

  return (
    <div className="workbench-shell" data-testid="workbench-shell">
      <aside className="workbench-sidebar" aria-label="Primary navigation">
        <div className="workbench-brand">
          <span className="workbench-brand-mark">TTS</span>
          <span className="workbench-brand-name">Trend Workbench</span>
        </div>
        <nav className="workbench-nav">
          <NavLink to="/trends" className={navClass} end>
            <TrendingUp size={16} aria-hidden /> Trending
          </NavLink>
          <NavLink to="/categories" className={navClass}>
            <Tags size={16} aria-hidden /> Categories
          </NavLink>
          <NavLink to="/imports" className={navClass}>
            <Upload size={16} aria-hidden /> Imports
          </NavLink>
        </nav>
        <div className="workbench-sidebar-footer" aria-hidden>
          <Menu size={14} />
        </div>
      </aside>
      <div className="workbench-main">
        <header className="workbench-header">
          <h1 className="workbench-title">{title}</h1>
          <span className="workbench-environment">Local workbench</span>
        </header>
        <main className="workbench-content" id="main" tabIndex={-1}>
          <Outlet />
        </main>
      </div>
    </div>
  )
}

function navClass({ isActive }: { isActive: boolean }): string {
  return `workbench-nav-link${isActive ? ' is-active' : ''}`
}