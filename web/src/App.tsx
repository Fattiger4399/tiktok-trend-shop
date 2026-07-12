import { Routes, Route, Navigate } from 'react-router-dom'
import WorkbenchShell from './components/WorkbenchShell'
import TrendingPage from './routes/TrendingPage'
import ProductDetailPage from './routes/ProductDetailPage'
import CategoriesPage from './routes/CategoriesPage'
import ImportsPage from './routes/ImportsPage'
import ImportDetailPage from './routes/ImportDetailPage'

export default function App() {
  return (
    <Routes>
      <Route element={<WorkbenchShell />}>
        <Route index element={<Navigate to="/trends" replace />} />
        <Route path="/trends" element={<TrendingPage />} />
        <Route path="/products/:id" element={<ProductDetailPage />} />
        <Route path="/categories" element={<CategoriesPage />} />
        <Route path="/imports" element={<ImportsPage />} />
        <Route path="/imports/:id" element={<ImportDetailPage />} />
        <Route path="*" element={<Navigate to="/trends" replace />} />
      </Route>
    </Routes>
  )
}