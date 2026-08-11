import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],

  server: {
    // ── Development proxy ──────────────────────────────────────────────────
    // Forwards any request starting with /api to the Go backend running
    // locally at http://localhost:8080.
    // Nginx handles this rewrite in production, so no code changes are needed
    // when deploying — both environments use the same /api/* URL pattern.
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        // Strip the /api prefix before forwarding to Go:
        //   /api/upload  →  http://localhost:8080/upload
        //   /api/status  →  http://localhost:8080/status
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
    },
  },
})
