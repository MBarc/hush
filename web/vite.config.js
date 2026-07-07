import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The built UI is embedded into the Go binary and served from the same
// origin, so API calls are same-origin in production. In dev, proxy to a
// locally running hush server.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:4874',
      '/healthz': 'http://localhost:4874',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
