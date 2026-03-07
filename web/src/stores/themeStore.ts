import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface ThemeState {
  dark: boolean
  toggle: () => void
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set) => ({
      dark: false,
      toggle: () =>
        set((state) => {
          const next = !state.dark
          if (next) {
            document.documentElement.classList.add('dark')
          } else {
            document.documentElement.classList.remove('dark')
          }
          return { dark: next }
        }),
    }),
    {
      name: 'starclaw-theme',
      onRehydrateStorage: () => (state) => {
        if (state?.dark) {
          document.documentElement.classList.add('dark')
        }
      },
    },
  ),
)
