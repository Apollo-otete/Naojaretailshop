// Admin console: separate Vite app and bundle, deployed standalone (subdomain or /admin).
// Output: dist-admin/index.html
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { resolve } from 'path';

export default defineConfig({
  root: 'admin',
  plugins: [react()],
  base: '/admin/',
  build: {
    outDir: '../dist-admin',
    emptyOutDir: true,
    rollupOptions: {
      input: resolve(__dirname, 'admin/index.html'),
    },
  },
  preview: {
    host: '127.0.0.1',
    allowedHosts: ['admin.naoja.local', 'localhost'],
  },
  server: {
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/uploads': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
});
