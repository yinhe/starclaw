import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3099,
    proxy: {
      '/call': { target: 'http://localhost:8099', changeOrigin: true },
      '/campaign': { target: 'http://localhost:8099', changeOrigin: true },
      '/callback': { target: 'http://localhost:8099', changeOrigin: true },
      '/customers': { target: 'http://localhost:8099', changeOrigin: true },
      '/calls': { target: 'http://localhost:8099', changeOrigin: true },
      '/scripts': { target: 'http://localhost:8099', changeOrigin: true },
      '/analytics': { target: 'http://localhost:8099', changeOrigin: true },
      '/recordings': { target: 'http://localhost:8099', changeOrigin: true },
      '/compliance': { target: 'http://localhost:8099', changeOrigin: true },
      '/health': { target: 'http://localhost:8099', changeOrigin: true },
      '/stats': { target: 'http://localhost:8099', changeOrigin: true },
    },
  },
})
