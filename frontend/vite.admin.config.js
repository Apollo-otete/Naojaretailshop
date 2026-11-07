// Admin console build: a separate Vite bundle so the admin app is not shipped inside the storefront.
// Output: dist-admin/ (index.html). Deploy behind its own /admin path or subdomain (nginx try_files).
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { resolve } from 'path';

export default defineConfig({
  plugins: [react()],
  base: '/admin/', // serve at /admin/* in production; dev still uses Vite paths
  build: {
    outDir: 'dist-admin',
    emptyOutDir: true,
    rollupOptions: {
      input: resolve(__dirname, 'admin.html'),
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/uploads': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
});
