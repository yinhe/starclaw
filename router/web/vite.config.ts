import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react-swc'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 3096,
    proxy: {
      '/auth': 'http://localhost:8096',
      '/dash': 'http://localhost:8096',
      '/pay': 'http://localhost:8096',
      '/health': 'http://localhost:8096',
    },
  },
})
