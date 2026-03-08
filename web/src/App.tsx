import { Routes, Route, Navigate } from 'react-router-dom'
import ErrorBoundary from './components/ErrorBoundary'
import ToastContainer from './components/ToastContainer'
import CommandPalette from './components/CommandPalette'
import { useAuthStore } from './stores/authStore'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import ChatPage from './pages/ChatPage'
import AgentsPage from './pages/AgentsPage'
import AgentDetailPage from './pages/AgentDetailPage'
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
import WorkflowTemplatePage from './pages/WorkflowTemplatePage'
import CodingAgentPage from './pages/CodingAgentPage'
import TasksPage from './pages/TasksPage'
import VisualizationPage from './pages/VisualizationPage'
import SkillsPage from './pages/SkillsPage'
import VideosPage from './pages/VideosPage'
import ResourcesPage from './pages/ResourcesPage'
import BillingPage from './pages/BillingPage'

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token)
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <ErrorBoundary>
    <ToastContainer />
    <CommandPalette />
    <Routes>
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
        <Route path="marketplace" element={<MarketplacePage />} />
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
        <Route path="visualization" element={<VisualizationPage />} />
        <Route path="billing" element={<BillingPage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
    </ErrorBoundary>
  )
}
