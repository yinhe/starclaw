import { create } from 'zustand'

interface User {
  id: string
  email: string
  username: string
  avatar: string
}

interface AuthState {
  token: string | null
  user: User | null
  setAuth: (token: string, user: User) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  token: localStorage.getItem('starclaw_token'),
  user: JSON.parse(localStorage.getItem('starclaw_user') || 'null'),
  setAuth: (token, user) => {
    localStorage.setItem('starclaw_token', token)
    localStorage.setItem('starclaw_user', JSON.stringify(user))
    set({ token, user })
  },
  logout: () => {
    localStorage.removeItem('starclaw_token')
    localStorage.removeItem('starclaw_user')
    set({ token: null, user: null })
  },
}))
