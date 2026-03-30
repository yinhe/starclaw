import { Routes, Route, Navigate } from 'react-router-dom'
import { useState, useEffect } from 'react'
import Landing from './pages/Landing'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'

function App() {
  const [token, setToken] = useState<string | null>(
    localStorage.getItem('ext_token')
  )

  useEffect(() => {
    if (token) localStorage.setItem('ext_token', token)
    else localStorage.removeItem('ext_token')
  }, [token])

  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route path="/login" element={<Login onLogin={setToken} />} />
      <Route
        path="/dashboard/*"
        element={token ? <Dashboard token={token} onLogout={() => setToken(null)} /> : <Navigate to="/login" />}
      />
    </Routes>
  )
}

export default App
