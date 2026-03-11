import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react-swc'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5174,
    proxy: {
      '/api/forum': {
        target: 'http://localhost:8093',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/forum/, '/forum'),
      },
      '/api/arena': {
        target: 'http://localhost:8094',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/arena/, '/arena'),
      },
      '/api/bounty': {
        target: 'http://localhost:8092',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/bounty/, '/bounties'),
      },
      '/api': {
        target: 'http://localhost:8085',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, '/v1'),
      },
    },
  },
})
