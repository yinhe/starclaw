import { Routes, Route, Navigate } from 'react-router-dom'
import { isLoggedIn } from './api'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import NodesPage from './pages/NodesPage'
import BillingStatsPage from './pages/BillingStatsPage'
import OrdersPage from './pages/OrdersPage'
import BalancesPage from './pages/BalancesPage'
import PackagesPage from './pages/PackagesPage'
import UsersPage from './pages/UsersPage'
import ReportsPage from './pages/ReportsPage'
import ServicesPage from './pages/ServicesPage'
import ReviewsPage from './pages/ReviewsPage'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  if (!isLoggedIn()) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route index element={<DashboardPage />} />
        <Route path="nodes" element={<NodesPage />} />
        <Route path="billing" element={<BillingStatsPage />} />
        <Route path="orders" element={<OrdersPage />} />
        <Route path="balances" element={<BalancesPage />} />
        <Route path="packages" element={<PackagesPage />} />
        <Route path="users" element={<UsersPage />} />
        <Route path="reports" element={<ReportsPage />} />
        <Route path="reviews" element={<ReviewsPage />} />
        <Route path="services" element={<ServicesPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
