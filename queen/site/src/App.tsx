import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { I18nProvider } from './i18n'
import { LandingPage } from './pages/LandingPage'
import { DocsPage } from './pages/DocsPage'
import { DownloadPage } from './pages/DownloadPage'
import { PricingPage } from './pages/PricingPage'
import { EnterprisePage } from './pages/EnterprisePage'
import { StarAIPage } from './pages/StarAIPage'
import { PartnersPage } from './pages/PartnersPage'
import { AboutPage } from './pages/AboutPage'

export default function App() {
  return (
    <I18nProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<LandingPage />} />
          <Route path="/docs" element={<DocsPage />} />
          <Route path="/download" element={<DownloadPage />} />
          <Route path="/pricing" element={<PricingPage />} />
          <Route path="/enterprise" element={<EnterprisePage />} />
          <Route path="/star-ai" element={<StarAIPage />} />
          <Route path="/partners" element={<PartnersPage />} />
          <Route path="/about" element={<AboutPage />} />
        </Routes>
      </BrowserRouter>
    </I18nProvider>
  )
}
