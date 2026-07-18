import { Routes, Route, Navigate } from 'react-router-dom'
import { RequireAuth } from './auth/RequireAuth'
import WorkbenchShell from './components/WorkbenchShell'
import LoginPage from './routes/LoginPage'
import TrendingPage from './routes/TrendingPage'
import ProductDetailPage from './routes/ProductDetailPage'
import CategoriesPage from './routes/CategoriesPage'
import ImportsPage from './routes/ImportsPage'
import ImportDetailPage from './routes/ImportDetailPage'
import RequestsPage from './routes/RequestsPage'
import RequestNewPage from './routes/RequestNewPage'
import RequestDetailPage from './routes/RequestDetailPage'
import DeliveriesPage from './routes/DeliveriesPage'

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        element={
          <RequireAuth>
            <WorkbenchShell />
          </RequireAuth>
        }
      >
        <Route index element={<Navigate to="/trends" replace />} />
        <Route path="/trends" element={<TrendingPage />} />
        <Route path="/products/:id" element={<ProductDetailPage />} />
        <Route path="/categories" element={<CategoriesPage />} />
        <Route path="/imports" element={<ImportsPage />} />
        <Route path="/imports/:id" element={<ImportDetailPage />} />
        <Route path="/requests" element={<RequestsPage />} />
        <Route path="/requests/new" element={<RequestNewPage />} />
        <Route path="/requests/:id" element={<RequestDetailPage />} />
        <Route path="/deliveries" element={<DeliveriesPage />} />
        <Route path="*" element={<Navigate to="/trends" replace />} />
      </Route>
    </Routes>
  )
}
