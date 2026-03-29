import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { LandingPage } from './pages/LandingPage';
import { AuthPage } from './pages/AuthPage';
import { DashboardPage } from './pages/DashboardPage';
import { MarketplacePage } from './pages/MarketplacePage';
import { MarketplaceDetailPage } from './pages/MarketplaceDetailPage';
import { DocsPage } from './pages/DocsPage';
import { ForumPage } from './pages/ForumPage';
import { ArenaPage } from './pages/ArenaPage';
import { BountyPage } from './pages/BountyPage';
import { BillingPage } from './pages/BillingPage';
import { DeveloperPage } from './pages/DeveloperPage';
import { ClawLoginPage } from './pages/ClawLoginPage';
import { InvestPage } from './pages/InvestPage';
import { CloudPage } from './pages/CloudPage';
import { GrowthPage } from './pages/GrowthPage';
import { ChrysalisPage } from './pages/ChrysalisPage';

const isInvestDomain = window.location.hostname === 'invest.starclaw.net';

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={isInvestDomain ? <InvestPage /> : <LandingPage />} />
        <Route path="/auth" element={<AuthPage />} />
        <Route path="/auth/claw-login" element={<ClawLoginPage />} />
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/marketplace" element={<MarketplacePage />} />
        <Route path="/marketplace/item/:id" element={<MarketplaceDetailPage />} />
        <Route path="/marketplace/:type" element={<MarketplacePage />} />
        <Route path="/docs" element={<DocsPage />} />
        <Route path="/forum" element={<ForumPage />} />
        <Route path="/arena" element={<ArenaPage />} />
        <Route path="/bounty" element={<BountyPage />} />
        <Route path="/billing" element={<BillingPage />} />
        <Route path="/developer" element={<DeveloperPage />} />
        <Route path="/invest" element={<InvestPage />} />
        <Route path="/cloud" element={<CloudPage />} />
        <Route path="/growth" element={<GrowthPage />} />
        <Route path="/chrysalis" element={<ChrysalisPage />} />
      </Routes>
    </BrowserRouter>
  );
}
