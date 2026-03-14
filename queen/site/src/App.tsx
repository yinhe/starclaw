import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { I18nProvider } from './i18n'
import { LandingPage } from './pages/LandingPage'
import { DocsPage } from './pages/DocsPage'
import { DownloadPage } from './pages/DownloadPage'

export default function App() {
  return (
    <I18nProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<LandingPage />} />
          <Route path="/docs" element={<DocsPage />} />
          <Route path="/download" element={<DownloadPage />} />
        </Routes>
      </BrowserRouter>
    </I18nProvider>
  )
}
