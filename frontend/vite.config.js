import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  // Remove the old esbuild config. Vite 8 uses Oxc, which automatically handles .jsx files.
  // If your component files have JSX in them, ensure they end in .jsx
  optimizeDeps: {
    // The old esbuildOptions is deprecated in Vite 8. 
    // If you MUST keep JSX in .js files, you need to tell the new engine.
    // However, the best practice is to rename your files to .jsx.
    // We will leave this empty to avoid the deprecation warnings.
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:5000',
        changeOrigin: true,
      },
      '/uploads': {
        target: 'http://localhost:5000',
        changeOrigin: true,
      },
    },
  },
});