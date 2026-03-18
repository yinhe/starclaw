import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  base: '/app/',
  plugins: [react()],
  server: {
    port: 3096,
    proxy: {
      '/brood': {
        target: 'http://localhost:8095',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://localhost:8095',
        changeOrigin: true,
      },
    },
  },
})
