import { create } from 'zustand'
import { configAPI } from '../lib/api'

interface ConfigState {
  deployMode: string // 'hosted' | 'opensource'
  loaded: boolean
  fetchConfig: () => Promise<void>
}

export const useConfigStore = create<ConfigState>((set) => ({
  deployMode: 'opensource',
  loaded: false,
  fetchConfig: async () => {
    try {
      const res = await configAPI.get()
      set({ deployMode: res.data.deploy_mode || 'opensource', loaded: true })
    } catch {
      set({ loaded: true })
    }
  },
}))
