import { useState, useEffect } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import ErrorBoundary from './components/ErrorBoundary'
import ToastContainer from './components/ToastContainer'
import CommandPalette from './components/CommandPalette'
import { useAuthStore } from './stores/authStore'
import { setupAPI } from './lib/api'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import SetupPage from './pages/SetupPage'
import ChatPage from './pages/ChatPage'
import AgentsPage from './pages/AgentsPage'
import AgentDetailPage from './pages/AgentDetailPage'
import TeamDetailPage from './pages/TeamDetailPage'
import ModelsPage from './pages/ModelsPage'
import WorkflowPage from './pages/WorkflowPage'
import WorkflowListPage from './pages/WorkflowListPage'
import KnowledgeBasePage from './pages/KnowledgeBasePage'
import MCPPage from './pages/MCPPage'
import MultiAgentPage from './pages/MultiAgentPage'
import DashboardPage from './pages/DashboardPage'
import SettingsPage from './pages/SettingsPage'
import NotFoundPage from './pages/NotFoundPage'
import MarketplacePage from './pages/MarketplacePage'
import MarketplaceDetailPage from './pages/MarketplaceDetailPage'
import WorkflowTemplatePage from './pages/WorkflowTemplatePage'
import CodingAgentPage from './pages/CodingAgentPage'
import TasksPage from './pages/TasksPage'
import ActivityPage from './pages/ActivityPage'
import VisualizationPage from './pages/VisualizationPage'
import SkillsPage from './pages/SkillsPage'
import VideosPage from './pages/VideosPage'
import ResourcesPage from './pages/ResourcesPage'
import BillingPage from './pages/BillingPage'
import WalletPage from './pages/WalletPage'
import MiningPage from './pages/MiningPage'
import IntegrationsPage from './pages/IntegrationsPage'
import SquadPage from './pages/SquadPage'
import MemoryPage from './pages/MemoryPage'
import ObservePage from './pages/ObservePage'
import WebhooksPage from './pages/WebhooksPage'
import DeveloperPage from './pages/DeveloperPage'
import SecurityPage from './pages/SecurityPage'
import GoalsPage from './pages/GoalsPage'
import FineTunePage from './pages/FineTunePage'
import HiveMindPage from './pages/HiveMindPage'

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token)
  const [setupDone, setSetupDone] = useState<boolean | null>(null)
  const [deployMode, setDeployMode] = useState<string>('opensource')

  useEffect(() => {
    setupAPI.status()
      .then((res) => {
        setSetupDone(res.data.setup_completed)
        setDeployMode(res.data.deploy_mode || 'opensource')
      })
      .catch(() => setSetupDone(true)) // on error, assume setup done (fallback)
  }, [])

  // Loading setup status
  if (setupDone === null) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="text-center">
          <div className="inline-flex items-center justify-center w-12 h-12 bg-primary-100 rounded-xl mb-3">
            <span className="text-2xl">🦞</span>
          </div>
          <p className="text-gray-400 text-sm">Loading...</p>
        </div>
      </div>
    )
  }

  // Setup not completed → redirect to setup page
  if (!setupDone && deployMode === 'opensource') {
    return <Navigate to="/setup" replace />
  }

  // No token → redirect to login
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <ErrorBoundary>
    <ToastContainer />
    <CommandPalette />
    <Routes>
      <Route path="/setup" element={<SetupPage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/"
        element={
          <PrivateRoute>
            <Layout />
          </PrivateRoute>
        }
      >
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<DashboardPage />} />
        <Route path="chat/:conversationId?" element={<ChatPage />} />
        <Route path="agents" element={<AgentsPage />} />
        <Route path="agents/:id" element={<AgentDetailPage />} />
        <Route path="teams/:id" element={<TeamDetailPage />} />
        <Route path="marketplace" element={<MarketplacePage />} />
        <Route path="marketplace/:id" element={<MarketplaceDetailPage />} />
        <Route path="models" element={<ModelsPage />} />
        <Route path="knowledge" element={<KnowledgeBasePage />} />
        <Route path="mcp" element={<MCPPage />} />
        <Route path="skills" element={<SkillsPage />} />
        <Route path="videos" element={<VideosPage />} />
        <Route path="resources" element={<ResourcesPage />} />
        <Route path="multi-agent" element={<MultiAgentPage />} />
        <Route path="workflows" element={<WorkflowListPage />} />
        <Route path="workflows/editor" element={<WorkflowPage />} />
        <Route path="workflows/templates" element={<WorkflowTemplatePage />} />
        <Route path="coding" element={<CodingAgentPage />} />
        <Route path="tasks" element={<TasksPage />} />
        <Route path="activities" element={<ActivityPage />} />
        <Route path="visualization" element={<VisualizationPage />} />
        <Route path="billing" element={<BillingPage />} />
        <Route path="wallet" element={<WalletPage />} />
        <Route path="mining" element={<MiningPage />} />
        <Route path="integrations" element={<IntegrationsPage />} />
        <Route path="squads" element={<SquadPage />} />
        <Route path="hivemind" element={<HiveMindPage />} />
        <Route path="memories" element={<MemoryPage />} />
        <Route path="observe" element={<ObservePage />} />
        <Route path="webhooks" element={<WebhooksPage />} />
        <Route path="developer" element={<DeveloperPage />} />
        <Route path="security" element={<SecurityPage />} />
        <Route path="goals" element={<GoalsPage />} />
        <Route path="finetune" element={<FineTunePage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
    </ErrorBoundary>
  )
}
