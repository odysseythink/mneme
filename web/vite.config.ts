import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      '/api':       { target: 'http://localhost:18801', changeOrigin: true },
      '/ws':        { target: 'ws://localhost:18801', ws: true },
      '/health':    { target: 'http://localhost:18801', changeOrigin: true },
      '/dev-token': { target: 'http://localhost:18801', changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
