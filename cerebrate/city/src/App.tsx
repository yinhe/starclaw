import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { isLoggedIn } from './lib/api'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import ClientsPage from './pages/ClientsPage'
import CommissionsPage from './pages/CommissionsPage'
import MaterialsPage from './pages/MaterialsPage'
import ClientStatsPage from './pages/ClientStatsPage'
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
          <Route path="/invites" element={<InvitesPage />} />
          <Route path="/clients" element={<ClientsPage />} />
          <Route path="/commissions" element={<CommissionsPage />} />
          <Route path="/client-stats" element={<ClientStatsPage />} />
          <Route path="/materials" element={<MaterialsPage />} />
          <Route path="/option" element={<OptionPage />} />
          <Route path="/ref-link" element={<MaterialsPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
