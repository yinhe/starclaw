import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { LandingPage } from './pages/LandingPage';
import { AuthPage } from './pages/AuthPage';
import { DashboardPage } from './pages/DashboardPage';
import { MarketplacePage } from './pages/MarketplacePage';
import { DocsPage } from './pages/DocsPage';
import { ForumPage } from './pages/ForumPage';
import { ArenaPage } from './pages/ArenaPage';
import { BountyPage } from './pages/BountyPage';
import { BillingPage } from './pages/BillingPage';

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<LandingPage />} />
        <Route path="/auth" element={<AuthPage />} />
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/marketplace" element={<MarketplacePage />} />
        <Route path="/marketplace/:type" element={<MarketplacePage />} />
        <Route path="/docs" element={<DocsPage />} />
        <Route path="/forum" element={<ForumPage />} />
        <Route path="/arena" element={<ArenaPage />} />
        <Route path="/bounty" element={<BountyPage />} />
        <Route path="/billing" element={<BillingPage />} />
      </Routes>
    </BrowserRouter>
  );
}
