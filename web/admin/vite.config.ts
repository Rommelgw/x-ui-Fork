import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8085',
        changeOrigin: true
      }
    }
  },
  preview: {
    port: 4173,
    strictPort: true
  }
});

