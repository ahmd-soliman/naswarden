import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  build: {
    // Build straight into internal/web/dist so `go:embed` can pick it up
    // with no separate copy step.
    outDir: '../internal/web/dist',
    emptyOutDir: true,
  },
  server: {
    // Local dev only: Vite serves the UI on its own port, the Go backend
    // (go run ./cmd/naswarden) serves /ws separately -- proxy it through
    // so the frontend can always just connect to its own origin. In
    // production the Go binary serves both from the same port, embedded
    // via go:embed, so this proxy is irrelevant there.
    proxy: {
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
})
