import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  root: 'src',
  base: './',
  build: {
    outDir: '../',
    emptyOutDir: false,
  },
  plugins: [react()],
})
