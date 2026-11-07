import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { HelmetProvider } from 'react-helmet-async';
import AdminApp from './AdminApp.jsx';
import '../index.css';

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <HelmetProvider>
      <AdminApp />
    </HelmetProvider>
  </StrictMode>
);
