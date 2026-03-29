import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { isLoggedIn } from './lib/api'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import DealsPage from './pages/DealsPage'
import CitiesPage from './pages/CitiesPage'
import CommissionsPage from './pages/CommissionsPage'
import EquityPage from './pages/EquityPage'
import NodesPage from './pages/NodesPage'
import DeployPage from './pages/DeployPage'
import InvitesPage from './pages/InvitesPage'
import OptionPage from './pages/OptionPage'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  return isLoggedIn() ? <>{children}</> : <Navigate to="/login" replace />
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          element={
            <ProtectedRoute>
              <Layout />
            </ProtectedRoute>
          }
        >
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/deals" element={<DealsPage />} />
          <Route path="/nodes" element={<NodesPage />} />
          <Route path="/invites" element={<InvitesPage />} />
          <Route path="/cities" element={<CitiesPage />} />
          <Route path="/commissions" element={<CommissionsPage />} />
          <Route path="/equity" element={<EquityPage />} />
          <Route path="/option" element={<OptionPage />} />
          <Route path="/deploy" element={<DeployPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
