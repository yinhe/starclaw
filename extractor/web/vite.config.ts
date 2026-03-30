import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5180,
    proxy: {
      '/api': {
        target: 'http://localhost:8097',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
      '/bridge': {
        target: 'http://localhost:8098',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/bridge/, ''),
      },
    },
  },
})
