import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react-swc'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 3097,
    proxy: {
      '/auth': 'http://localhost:8096',
      '/admin': 'http://localhost:8096',
    },
  },
})
